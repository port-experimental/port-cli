"""Export module implementation for exporting Port data."""

import json
import tarfile
from pathlib import Path
from typing import Any, Dict, List, Optional

from rich.console import Console
from rich.progress import Progress, SpinnerColumn, TextColumn

from port_cli.api_client import PortAPIClient
from port_cli.modules.base import BaseModule, ModuleResult

console = Console(stderr=True)


class ExportModule(BaseModule):
    """
    Module for exporting data from Port.
    
    Exports blueprints, entities, scorecards, actions, and teams
    to JSON or tar.gz format.
    """

    def validate(self, **kwargs: Any) -> bool:
        """Validate export parameters."""
        output_path = kwargs.get("output_path")
        if not output_path:
            raise ValueError("output_path is required")

        format_type = kwargs.get("format", "tar")
        if format_type not in ["json", "tar"]:
            raise ValueError("format must be 'json' or 'tar'")

        return True

    def execute(self, **kwargs: Any) -> ModuleResult:
        """
        Execute the export operation.
        
        Args:
            output_path: Path to output file
            blueprints: Optional list of blueprint IDs to export
            format: Output format ('json' or 'tar')
            skip_entities: Skip exporting entities (default: False)
            include_resources: Optional list of resource types to export (default: all)
            
        Returns:
            ModuleResult with export outcome
        """
        try:
            self.validate(**kwargs)

            output_path = Path(kwargs["output_path"])
            blueprint_filter = kwargs.get("blueprints")
            format_type = kwargs.get("format", "tar")
            skip_entities = kwargs.get("skip_entities", False)
            include_resources = kwargs.get("include_resources")

            # Create API client
            client = PortAPIClient(
                client_id=self.config.client_id,
                client_secret=self.config.client_secret,
                api_url=self.config.api_url,
            )

            with client:
                # Collect data
                data = self._collect_data(client, blueprint_filter, skip_entities, include_resources)

                # Write output
                if format_type == "tar":
                    self._write_tar(data, output_path)
                else:
                    self._write_json(data, output_path)

            return ModuleResult(
                success=True,
                message=f"Successfully exported data to {output_path}",
                data={
                    "output_path": str(output_path),
                    "blueprints_count": len(data["blueprints"]),
                    "entities_count": len(data["entities"]),
                    "automations_count": len(data["automations"]),
                    "pages_count": len(data["pages"]),
                    "integrations_count": len(data["integrations"]),
                },
            )

        except Exception as e:
            return ModuleResult(
                success=False,
                message="Export failed",
                error=str(e),
            )

    def _collect_data(
        self, 
        client: PortAPIClient, 
        blueprint_filter: Optional[List[str]], 
        skip_entities: bool = False,
        include_resources: Optional[List[str]] = None
    ) -> Dict[str, Any]:
        """
        Collect all data from Port.
        
        Args:
            client: Port API client
            blueprint_filter: Optional list of blueprint IDs to filter
            skip_entities: If True, skip entity collection
            include_resources: Optional list of resource types to include (if None, include all)
        """
        data: Dict[str, Any] = {
            "blueprints": [],
            "entities": [],
            "scorecards": [],
            "actions": [],
            "teams": [],
            "automations": [],
            "pages": [],
            "integrations": [],
        }

        # Helper to check if a resource should be collected
        def should_collect(resource_type: str) -> bool:
            if include_resources is None:
                return True
            return resource_type in include_resources

        # Get all blueprints
        if should_collect("blueprints"):
            all_blueprints = client.get_blueprints()

            # Filter blueprints if specified
            if blueprint_filter:
                blueprints = [
                    bp for bp in all_blueprints if bp.get("identifier") in blueprint_filter
                ]
            else:
                blueprints = all_blueprints

            data["blueprints"] = blueprints
        else:
            blueprints = []

        # Get entities and related data for each blueprint
        # Note: We need blueprints to get entities/scorecards/actions even if blueprints aren't included
        if not blueprints and (should_collect("entities") or should_collect("scorecards") or should_collect("actions")):
            # Need to fetch blueprints for dependent resources
            all_blueprints = client.get_blueprints()
            if blueprint_filter:
                blueprints = [
                    bp for bp in all_blueprints if bp.get("identifier") in blueprint_filter
                ]
            else:
                blueprints = all_blueprints

        for blueprint in blueprints:
            blueprint_id = blueprint.get("identifier")
            if blueprint_id:
                try:
                    # Collect entities if not skipped and should be included
                    if not skip_entities and should_collect("entities"):
                        entities = client.get_entities(blueprint_id)
                        data["entities"].extend(entities)

                    # Get scorecards
                    if should_collect("scorecards"):
                        scorecards = client.get_scorecards(blueprint_id)
                        data["scorecards"].extend(scorecards)

                    # Get actions
                    if should_collect("actions"):
                        actions = client.get_actions(blueprint_id)
                        data["actions"].extend(actions)
                except Exception as e:
                    # Only warn for unexpected errors (410 Gone is expected for blueprints without actions)
                    error_msg = str(e)
                    if "410 Gone" in error_msg and "actions" in error_msg:
                        # Silent skip for expected 410 on actions endpoint
                        pass
                    else:
                        console.print(f"[yellow]⚠[/yellow] Failed to get data for [cyan]{blueprint_id}[/cyan]: {error_msg}")

        # Get teams
        if should_collect("teams"):
            try:
                teams = client.get_teams()
                data["teams"] = teams
            except Exception as e:
                console.print(f"[yellow]⚠[/yellow] Failed to get teams: {e}")

        # Get all actions/automations (organization-wide)
        if should_collect("automations"):
            try:
                all_actions = client.get_all_actions()
                # Filter to only automations (actions have been collected per-blueprint already)
                # Automations typically have trigger properties, but we'll store all for now
                data["automations"] = all_actions
            except Exception as e:
                console.print(f"[yellow]⚠[/yellow] Failed to get automations: {e}")

        # Get pages
        if should_collect("pages"):
            try:
                pages = client.get_pages()
                data["pages"] = pages
            except Exception as e:
                console.print(f"[yellow]⚠[/yellow] Failed to get pages: {e}")

        # Get integrations
        if should_collect("integrations"):
            try:
                integrations = client.get_integrations()
                data["integrations"] = integrations
            except Exception as e:
                console.print(f"[yellow]⚠[/yellow] Failed to get integrations: {e}")

        return data

    def _write_json(self, data: Dict[str, Any], output_path: Path) -> None:
        """Write data to JSON file."""
        output_path.parent.mkdir(parents=True, exist_ok=True)
        
        with open(output_path, "w") as f:
            json.dump(data, f, indent=2)

    def _write_tar(self, data: Dict[str, Any], output_path: Path) -> None:
        """Write data to tar.gz file."""
        output_path.parent.mkdir(parents=True, exist_ok=True)

        with tarfile.open(output_path, "w:gz") as tar:
            # Write each data type to separate JSON files in the tar
            for data_type, items in data.items():
                json_data = json.dumps(items, indent=2).encode("utf-8")
                
                # Create tarfile info
                import io
                tarinfo = tarfile.TarInfo(name=f"{data_type}.json")
                tarinfo.size = len(json_data)
                
                # Add to tar
                tar.addfile(tarinfo, io.BytesIO(json_data))

