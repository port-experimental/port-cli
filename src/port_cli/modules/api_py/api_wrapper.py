"""API wrapper module implementation for direct Port API operations."""

from typing import Any, Dict, Optional

from port_cli.api_client import PortAPIClient
from port_cli.modules.base import BaseModule, ModuleResult


class APIModule(BaseModule):
    """
    Module for direct Port API operations.
    
    Provides CRUD operations for blueprints, entities, and other resources.
    """

    def validate(self, **kwargs: Any) -> bool:
        """Validate API operation parameters."""
        operation = kwargs.get("operation")
        if not operation:
            raise ValueError("operation is required")

        return True

    def execute(self, **kwargs: Any) -> ModuleResult:
        """
        Execute an API operation.
        
        Args:
            operation: Operation to perform (e.g., 'blueprints.list', 'entities.create')
            **kwargs: Operation-specific parameters
            
        Returns:
            ModuleResult with operation outcome
        """
        try:
            self.validate(**kwargs)

            operation = kwargs["operation"]
            
            # Create API client
            client = PortAPIClient(
                client_id=self.config.client_id,
                client_secret=self.config.client_secret,
                api_url=self.config.api_url,
            )

            with client:
                result_data = self._execute_operation(client, operation, kwargs)

            return ModuleResult(
                success=True,
                message=f"Operation {operation} completed successfully",
                data=result_data,
            )

        except Exception as e:
            return ModuleResult(
                success=False,
                message=f"Operation failed",
                error=str(e),
            )

    def _execute_operation(
        self, client: PortAPIClient, operation: str, params: Dict[str, Any]
    ) -> Any:
        """Execute the specified operation."""
        # Parse operation (e.g., "blueprints.list" -> resource="blueprints", action="list")
        parts = operation.split(".")
        if len(parts) != 2:
            raise ValueError(f"Invalid operation format: {operation}")

        resource, action = parts

        if resource == "blueprints":
            return self._blueprints_operation(client, action, params)
        elif resource == "entities":
            return self._entities_operation(client, action, params)
        else:
            raise ValueError(f"Unknown resource: {resource}")

    def _blueprints_operation(
        self, client: PortAPIClient, action: str, params: Dict[str, Any]
    ) -> Any:
        """Handle blueprint operations."""
        if action == "list":
            return {"blueprints": client.get_blueprints()}
        
        elif action == "get":
            identifier = params.get("identifier")
            if not identifier:
                raise ValueError("identifier is required for get operation")
            return {"blueprint": client.get_blueprint(identifier)}
        
        elif action == "create":
            data = params.get("data")
            if not data:
                raise ValueError("data is required for create operation")
            return {"blueprint": client.create_blueprint(data)}
        
        elif action == "update":
            identifier = params.get("identifier")
            data = params.get("data")
            if not identifier or not data:
                raise ValueError("identifier and data are required for update operation")
            return {"blueprint": client.update_blueprint(identifier, data)}
        
        elif action == "delete":
            identifier = params.get("identifier")
            if not identifier:
                raise ValueError("identifier is required for delete operation")
            client.delete_blueprint(identifier)
            return {"status": "deleted"}
        
        else:
            raise ValueError(f"Unknown action: {action}")

    def _entities_operation(
        self, client: PortAPIClient, action: str, params: Dict[str, Any]
    ) -> Any:
        """Handle entity operations."""
        if action == "list":
            blueprint_id = params.get("blueprint_identifier")
            if not blueprint_id:
                raise ValueError("blueprint_identifier is required for list operation")
            return {"entities": client.get_entities(blueprint_id)}
        
        elif action == "get":
            blueprint_id = params.get("blueprint_identifier")
            entity_id = params.get("identifier")
            if not blueprint_id or not entity_id:
                raise ValueError("blueprint_identifier and identifier are required")
            return {"entity": client.get_entity(blueprint_id, entity_id)}
        
        elif action == "create":
            blueprint_id = params.get("blueprint_identifier")
            data = params.get("data")
            if not blueprint_id or not data:
                raise ValueError("blueprint_identifier and data are required")
            return {"entity": client.create_entity(blueprint_id, data)}
        
        elif action == "update":
            blueprint_id = params.get("blueprint_identifier")
            entity_id = params.get("identifier")
            data = params.get("data")
            if not blueprint_id or not entity_id or not data:
                raise ValueError("blueprint_identifier, identifier, and data are required")
            return {"entity": client.update_entity(blueprint_id, entity_id, data)}
        
        elif action == "delete":
            blueprint_id = params.get("blueprint_identifier")
            entity_id = params.get("identifier")
            if not blueprint_id or not entity_id:
                raise ValueError("blueprint_identifier and identifier are required")
            client.delete_entity(blueprint_id, entity_id)
            return {"status": "deleted"}
        
        else:
            raise ValueError(f"Unknown action: {action}")

