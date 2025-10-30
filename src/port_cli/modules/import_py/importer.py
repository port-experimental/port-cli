"""Import module implementation for importing Port data."""

import json
import tarfile
from pathlib import Path
from typing import Any, Dict, List, Optional

from port_cli.api_client import PortAPIClient
from port_cli.modules.base import BaseModule, ModuleResult


class ImportModule(BaseModule):
    """
    Module for importing data to Port.
    
    Imports blueprints, entities, scorecards, actions, and teams
    from JSON or tar.gz format.
    """

    def validate(self, **kwargs: Any) -> bool:
        """Validate import parameters."""
        input_path = kwargs.get("input_path")
        if not input_path:
            raise ValueError("input_path is required")

        path = Path(input_path)
        if not path.exists():
            raise ValueError(f"Input file does not exist: {input_path}")

        return True

    def execute(self, **kwargs: Any) -> ModuleResult:
        """
        Execute the import operation.
        
        Args:
            input_path: Path to input file
            dry_run: If True, validate without applying changes
            conflict_handler: How to handle conflicts ('skip', 'overwrite', 'fail')
            skip_entities: Skip importing entities (default: False)
            include_resources: Optional list of resource types to import (default: all)
            
        Returns:
            ModuleResult with import outcome
        """
        try:
            self.validate(**kwargs)

            input_path = Path(kwargs["input_path"])
            dry_run = kwargs.get("dry_run", False)
            conflict_handler = kwargs.get("conflict_handler", "skip")
            skip_entities = kwargs.get("skip_entities", False)
            include_resources = kwargs.get("include_resources")

            # Load data
            data = self._load_data(input_path)

            # Validate data
            self._validate_data(data)

            if dry_run:
                return ModuleResult(
                    success=True,
                    message="Validation passed (dry run - no changes applied)",
                    data={
                        "blueprints": len(data.get("blueprints", [])),
                        "entities": len(data.get("entities", [])),
                    },
                )

            # Create API client
            client = PortAPIClient(
                client_id=self.config.client_id,
                client_secret=self.config.client_secret,
                api_url=self.config.api_url,
            )

            with client:
                # Import data
                result = self._import_data(client, data, conflict_handler, skip_entities, include_resources)

            return ModuleResult(
                success=True,
                message="Successfully imported data",
                data=result,
            )

        except Exception as e:
            return ModuleResult(
                success=False,
                message="Import failed",
                error=str(e),
            )

    def _load_data(self, input_path: Path) -> Dict[str, Any]:
        """Load data from file."""
        if input_path.suffix == ".json":
            with open(input_path, "r") as f:
                return json.load(f)
        elif input_path.suffix == ".gz" or ".tar" in input_path.suffixes:
            return self._load_tar(input_path)
        else:
            raise ValueError(f"Unsupported file format: {input_path.suffix}")

    def _load_tar(self, tar_path: Path) -> Dict[str, Any]:
        """Load data from tar.gz file."""
        data: Dict[str, Any] = {}

        with tarfile.open(tar_path, "r:gz") as tar:
            for member in tar.getmembers():
                if member.isfile() and member.name.endswith(".json"):
                    f = tar.extractfile(member)
                    if f:
                        content = f.read().decode("utf-8")
                        data_type = member.name.replace(".json", "")
                        data[data_type] = json.loads(content)

        return data

    def _validate_data(self, data: Dict[str, Any]) -> None:
        """Validate the loaded data structure."""
        required_keys = ["blueprints"]
        for key in required_keys:
            if key not in data:
                raise ValueError(f"Missing required data: {key}")

    def _import_data(
        self, 
        client: PortAPIClient, 
        data: Dict[str, Any], 
        conflict_handler: str,
        skip_entities: bool = False,
        include_resources: Optional[List[str]] = None
    ) -> Dict[str, Any]:
        """Import data to Port with selective resource support."""
        result = {
            "blueprints_created": 0,
            "blueprints_updated": 0,
            "entities_created": 0,
            "entities_updated": 0,
            "scorecards_created": 0,
            "scorecards_updated": 0,
            "actions_created": 0,
            "actions_updated": 0,
            "teams_created": 0,
            "teams_updated": 0,
            "automations_created": 0,
            "automations_updated": 0,
            "pages_created": 0,
            "pages_updated": 0,
            "integrations_updated": 0,
            "errors": [],
        }

        # Helper to check if a resource should be imported
        def should_import(resource_type: str) -> bool:
            if include_resources is None:
                return True
            return resource_type in include_resources

        # Import blueprints first
        if should_import("blueprints"):
            for blueprint in data.get("blueprints", []):
                identifier = blueprint.get("identifier")
                if not identifier:
                    continue
                    
                # Skip system blueprints (those starting with underscore)
                if identifier.startswith("_"):
                    continue

                try:
                    # Try to create blueprint first
                    client.create_blueprint(blueprint)
                    result["blueprints_created"] += 1
                except Exception as e:
                    error_msg = str(e)
                    # If blueprint exists (409 Conflict), always try to update/merge
                    if "409" in error_msg or "Conflict" in error_msg:
                        try:
                            client.update_blueprint(identifier, blueprint)
                            result["blueprints_updated"] += 1
                        except Exception as update_error:
                            result["errors"].append(f"Blueprint {identifier}: {str(update_error)}")
                    else:
                        # Other errors
                        result["errors"].append(f"Blueprint {identifier}: {error_msg}")

        # Import entities
        if not skip_entities and should_import("entities"):
            for entity in data.get("entities", []):
                blueprint_id = entity.get("blueprint")
                entity_id = entity.get("identifier")
                
                if not blueprint_id or not entity_id:
                    continue

                try:
                    # Try to create entity first
                    client.create_entity(blueprint_id, entity)
                    result["entities_created"] += 1
                except Exception as e:
                    error_msg = str(e)
                    # If entity exists (409 Conflict), always try to update/merge
                    if "409" in error_msg or "Conflict" in error_msg:
                        try:
                            client.update_entity(blueprint_id, entity_id, entity)
                            result["entities_updated"] += 1
                        except Exception as update_error:
                            result["errors"].append(f"Entity {entity_id}: {str(update_error)}")
                    else:
                        # Other errors
                        result["errors"].append(f"Entity {entity_id}: {error_msg}")

        # Import scorecards
        if should_import("scorecards"):
            for scorecard in data.get("scorecards", []):
                blueprint_id = scorecard.get("blueprintIdentifier")
                scorecard_id = scorecard.get("identifier")
                if not blueprint_id or not scorecard_id:
                    continue
                
                # Clean scorecard data - remove system fields
                cleaned_scorecard = {k: v for k, v in scorecard.items() 
                                    if k not in ["createdBy", "updatedBy", "createdAt", "updatedAt", "id"]}
                
                try:
                    # Try to create scorecard first
                    client.create_scorecard(blueprint_id, cleaned_scorecard)
                    result["scorecards_created"] += 1
                except Exception as e:
                    error_msg = str(e)
                    # If scorecard exists (409 Conflict), always try to update/merge
                    if "409" in error_msg or "Conflict" in error_msg:
                        try:
                            client.update_scorecard(blueprint_id, scorecard_id, cleaned_scorecard)
                            result["scorecards_updated"] += 1
                        except Exception as update_error:
                            result["errors"].append(f"Scorecard {scorecard_id}: {str(update_error)}")
                    else:
                        # Other errors
                        result["errors"].append(f"Scorecard {scorecard_id}: {error_msg}")

        # Import actions
        if should_import("actions"):
            for action in data.get("actions", []):
                blueprint_id = action.get("blueprintIdentifier")
                action_id = action.get("identifier")
                if not blueprint_id or not action_id:
                    continue
                
                # Clean action data - remove system fields
                cleaned_action = {k: v for k, v in action.items() 
                                 if k not in ["createdBy", "updatedBy", "createdAt", "updatedAt", "id"]}
                
                try:
                    # Try to create action first
                    client.create_action(blueprint_id, cleaned_action)
                    result["actions_created"] += 1
                except Exception as e:
                    error_msg = str(e)
                    # If action exists (409 Conflict), always try to update/merge
                    if "409" in error_msg or "Conflict" in error_msg:
                        try:
                            client.update_action(blueprint_id, action_id, cleaned_action)
                            result["actions_updated"] += 1
                        except Exception as update_error:
                            result["errors"].append(f"Action {action_id}: {str(update_error)}")
                    else:
                        # Other errors
                        result["errors"].append(f"Action {action_id}: {error_msg}")

        # Import teams
        if should_import("teams"):
            for team in data.get("teams", []):
                team_name = team.get("name")
                if not team_name:
                    continue
                try:
                    # Try to create team first
                    client.create_team(team)
                    result["teams_created"] += 1
                except Exception as e:
                    error_msg = str(e)
                    # If team exists (409 Conflict), always try to update/merge
                    if "409" in error_msg or "Conflict" in error_msg:
                        try:
                            client.update_team(team_name, team)
                            result["teams_updated"] += 1
                        except Exception as update_error:
                            result["errors"].append(f"Team {team_name}: {str(update_error)}")
                    else:
                        # Other errors
                        result["errors"].append(f"Team {team_name}: {error_msg}")

        # Import automations
        if should_import("automations"):
            for automation in data.get("automations", []):
                automation_id = automation.get("identifier")
                if not automation_id:
                    continue
                
                # Clean automation data - remove system fields that API doesn't accept
                cleaned_automation = {k: v for k, v in automation.items() 
                                     if k not in ["createdBy", "updatedBy", "createdAt", "updatedAt", "id"]}
                
                try:
                    # Try to create automation first
                    client.create_automation(cleaned_automation)
                    result["automations_created"] += 1
                except Exception as e:
                    error_msg = str(e)
                    # If automation exists (409 Conflict), always try to update/merge
                    if "409" in error_msg or "Conflict" in error_msg:
                        try:
                            client.update_automation(automation_id, cleaned_automation)
                            result["automations_updated"] += 1
                        except Exception as update_error:
                            result["errors"].append(f"Automation {automation_id}: {str(update_error)}")
                    else:
                        # Other errors (422, etc.)
                        result["errors"].append(f"Automation {automation_id}: {error_msg}")

        # Import pages
        if should_import("pages"):
            for page in data.get("pages", []):
                page_id = page.get("identifier")
                page_type = page.get("type")
                if not page_id:
                    continue
                
                # Skip system-managed pages that cannot be updated via API
                # These include entity pages, blueprint-entities pages, and system pages
                system_page_types = {"entity", "blueprint-entities", "home", "audit-log", 
                                    "runs-history", "user", "team", "run", "users-and-teams"}
                if page_type in system_page_types:
                    continue
                
                # Clean page data - remove system/read-only fields
                cleaned_page = {k: v for k, v in page.items() 
                               if k not in ["createdBy", "updatedBy", "createdAt", "updatedAt", "id", "protected", "after", "section", "sidebar"]}
                
                try:
                    # Try to create page first
                    client.create_page(cleaned_page)
                    result["pages_created"] += 1
                except Exception as e:
                    error_msg = str(e)
                    # If page exists (409 Conflict), always try to update/merge
                    if "409" in error_msg or "Conflict" in error_msg:
                        try:
                            client.update_page(page_id, cleaned_page)
                            result["pages_updated"] += 1
                        except Exception as update_error:
                            result["errors"].append(f"Page {page_id}: {str(update_error)}")
                    else:
                        # Other errors
                        result["errors"].append(f"Page {page_id}: {error_msg}")

        # Import integrations (update config only, as creation may not be supported)
        if should_import("integrations"):
            for integration in data.get("integrations", []):
                integration_id = integration.get("identifier")
                if not integration_id:
                    continue
                try:
                    # Only update config for integrations
                    client.update_integration_config(integration_id, integration)
                    result["integrations_updated"] += 1
                except Exception as e:
                    result["errors"].append(f"Integration {integration_id}: {str(e)}")

        return result

