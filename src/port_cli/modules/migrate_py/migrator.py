"""Migration module implementation for migrating between Port organizations."""

from typing import Any, Dict, List, Optional

from rich.console import Console

from port_cli.api_client import PortAPIClient
from port_cli.modules.base import BaseModule, ModuleResult

console = Console(stderr=True)


class MigrateModule(BaseModule):
    """
    Module for migrating data between Port organizations.
    
    Handles source and target coordination, dependency resolution,
    and progress tracking.
    """

    def __init__(self, source_config: Any, target_config: Any):
        """
        Initialize with both source and target configurations.
        
        Args:
            source_config: Source organization configuration
            target_config: Target organization configuration
        """
        self.source_config = source_config
        self.target_config = target_config
        # For base class compatibility
        super().__init__(source_config)

    def validate(self, **kwargs: Any) -> bool:
        """Validate migration parameters."""
        # Validate configs exist
        if not self.source_config or not self.target_config:
            raise ValueError("Both source and target configurations are required")

        return True

    def execute(self, **kwargs: Any) -> ModuleResult:
        """
        Execute the migration operation.
        
        Args:
            blueprints: Optional list of blueprint IDs to migrate
            dry_run: If True, validate without applying changes
            
        Returns:
            ModuleResult with migration outcome
        """
        try:
            self.validate(**kwargs)

            blueprint_filter = kwargs.get("blueprints")
            dry_run = kwargs.get("dry_run", False)

            # Create API clients
            source_client = PortAPIClient(
                client_id=self.source_config.client_id,
                client_secret=self.source_config.client_secret,
                api_url=self.source_config.api_url,
            )

            target_client = PortAPIClient(
                client_id=self.target_config.client_id,
                client_secret=self.target_config.client_secret,
                api_url=self.target_config.api_url,
            )

            with source_client, target_client:
                # Export from source
                source_data = self._export_from_source(source_client, blueprint_filter)

                if dry_run:
                    return ModuleResult(
                        success=True,
                        message="Migration validation passed (dry run)",
                        data={
                            "blueprints": len(source_data.get("blueprints", [])),
                            "entities": len(source_data.get("entities", [])),
                        },
                    )

                # Import to target
                result = self._import_to_target(target_client, source_data)

            return ModuleResult(
                success=True,
                message="Migration completed successfully",
                data=result,
            )

        except Exception as e:
            return ModuleResult(
                success=False,
                message="Migration failed",
                error=str(e),
            )

    def _export_from_source(
        self, client: PortAPIClient, blueprint_filter: Optional[List[str]]
    ) -> Dict[str, Any]:
        """Export data from source organization."""
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

        # Get blueprints
        all_blueprints = client.get_blueprints()

        if blueprint_filter:
            blueprints = [
                bp for bp in all_blueprints if bp.get("identifier") in blueprint_filter
            ]
        else:
            blueprints = all_blueprints

        # Resolve dependencies
        blueprints = self._resolve_dependencies(all_blueprints, blueprints)
        data["blueprints"] = blueprints

        # Get entities for each blueprint
        for blueprint in blueprints:
            blueprint_id = blueprint.get("identifier")
            if blueprint_id:
                try:
                    entities = client.get_entities(blueprint_id)
                    data["entities"].extend(entities)

                    scorecards = client.get_scorecards(blueprint_id)
                    data["scorecards"].extend(scorecards)

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
        try:
            teams = client.get_teams()
            data["teams"] = teams
        except Exception:
            pass

        # Get all actions/automations (organization-wide)
        try:
            all_actions = client.get_all_actions()
            # Store as automations (includes all org-level actions/automations)
            data["automations"] = all_actions
        except Exception as e:
            console.print(f"[yellow]⚠[/yellow] Failed to get automations: {e}")

        # Get pages
        try:
            pages = client.get_pages()
            data["pages"] = pages
        except Exception as e:
            console.print(f"[yellow]⚠[/yellow] Failed to get pages: {e}")

        # Get integrations
        try:
            integrations = client.get_integrations()
            data["integrations"] = integrations
        except Exception as e:
            console.print(f"[yellow]⚠[/yellow] Failed to get integrations: {e}")

        return data

    def _resolve_dependencies(
        self, all_blueprints: List[Dict[str, Any]], selected_blueprints: List[Dict[str, Any]]
    ) -> List[Dict[str, Any]]:
        """
        Resolve blueprint dependencies.
        
        If a blueprint has relations to other blueprints, ensure those
        blueprints are also included.
        """
        selected_ids = {bp["identifier"] for bp in selected_blueprints}
        all_blueprints_map = {bp["identifier"]: bp for bp in all_blueprints}
        
        result = list(selected_blueprints)
        to_check = list(selected_ids)
        checked = set()

        while to_check:
            blueprint_id = to_check.pop()
            if blueprint_id in checked:
                continue
            checked.add(blueprint_id)

            blueprint = all_blueprints_map.get(blueprint_id)
            if not blueprint:
                continue

            # Check relations
            relations = blueprint.get("relations", {})
            for relation_id, relation in relations.items():
                target = relation.get("target")
                if target and target not in selected_ids:
                    # Add dependency
                    if target in all_blueprints_map:
                        result.append(all_blueprints_map[target])
                        selected_ids.add(target)
                        to_check.append(target)

        return result

    def _import_to_target(
        self, client: PortAPIClient, data: Dict[str, Any]
    ) -> Dict[str, Any]:
        """Import data to target organization."""
        result = {
            "blueprints_created": 0,
            "blueprints_skipped": 0,
            "entities_created": 0,
            "entities_skipped": 0,
            "automations_exported": len(data.get("automations", [])),
            "pages_exported": len(data.get("pages", [])),
            "integrations_exported": len(data.get("integrations", [])),
            "errors": [],
        }

        # Get existing blueprints
        existing_blueprints = {bp["identifier"]: bp for bp in client.get_blueprints()}

        # Import blueprints
        for blueprint in data.get("blueprints", []):
            identifier = blueprint.get("identifier")
            if not identifier:
                continue

            try:
                if identifier in existing_blueprints:
                    result["blueprints_skipped"] += 1
                else:
                    client.create_blueprint(blueprint)
                    result["blueprints_created"] += 1
            except Exception as e:
                result["errors"].append(f"Blueprint {identifier}: {str(e)}")

        # Import entities
        for entity in data.get("entities", []):
            blueprint_id = entity.get("blueprint")
            entity_id = entity.get("identifier")
            
            if not blueprint_id or not entity_id:
                continue

            try:
                # Check if exists
                try:
                    client.get_entity(blueprint_id, entity_id)
                    result["entities_skipped"] += 1
                except:
                    client.create_entity(blueprint_id, entity)
                    result["entities_created"] += 1
            except Exception as e:
                result["errors"].append(f"Entity {entity_id}: {str(e)}")

        return result

