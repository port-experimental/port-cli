"""Port API client for making authenticated requests to Port's API."""

import time
from typing import Any, Dict, List, Optional

import httpx
from pydantic import BaseModel


class TokenResponse(BaseModel):
    """Port API token response."""

    accessToken: str
    expiresIn: int
    tokenType: str


class PortAPIClient:
    """
    Client for interacting with Port's API.
    
    Handles authentication, token management, and provides methods
    for all Port API operations.
    """

    def __init__(
        self,
        client_id: str,
        client_secret: str,
        api_url: str = "https://api.getport.io/v1",
        timeout: int = 300,
    ):
        """
        Initialize the Port API client.
        
        Args:
            client_id: Port API client ID
            client_secret: Port API client secret
            api_url: Port API base URL
            timeout: Request timeout in seconds
        """
        self.client_id = client_id
        self.client_secret = client_secret
        self.api_url = api_url.rstrip("/")
        self.timeout = timeout
        
        self._token: Optional[str] = None
        self._token_expiry: float = 0
        self._client = httpx.Client(timeout=timeout)

    def __enter__(self) -> "PortAPIClient":
        """Context manager entry."""
        return self

    def __exit__(self, *args: Any) -> None:
        """Context manager exit."""
        self.close()

    def close(self) -> None:
        """Close the HTTP client."""
        self._client.close()

    def _get_token(self) -> str:
        """Get or refresh the authentication token."""
        # Check if token is still valid (with 5 minute buffer)
        if self._token and time.time() < self._token_expiry - 300:
            return self._token

        # Request new token
        auth_url = f"{self.api_url}/auth/access_token"
        payload = {
            "clientId": self.client_id,
            "clientSecret": self.client_secret,
        }

        response = self._client.post(auth_url, json=payload)
        response.raise_for_status()

        token_data = TokenResponse(**response.json())
        self._token = token_data.accessToken
        self._token_expiry = time.time() + token_data.expiresIn

        return self._token

    def _request(
        self,
        method: str,
        path: str,
        data: Optional[Dict[str, Any]] = None,
        params: Optional[Dict[str, Any]] = None,
    ) -> httpx.Response:
        """
        Make an authenticated request to the Port API.
        
        Args:
            method: HTTP method
            path: API path (relative to base URL)
            data: Request body (for POST/PUT)
            params: Query parameters
            
        Returns:
            HTTP response
        """
        token = self._get_token()
        url = f"{self.api_url}{path}"
        
        headers = {
            "Authorization": f"Bearer {token}",
            "Content-Type": "application/json",
        }

        response = self._client.request(
            method=method,
            url=url,
            headers=headers,
            json=data,
            params=params,
        )
        response.raise_for_status()
        return response

    # Blueprint operations

    def get_blueprints(self) -> List[Dict[str, Any]]:
        """Get all blueprints."""
        response = self._request("GET", "/blueprints")
        return response.json().get("blueprints", [])

    def get_blueprint(self, identifier: str) -> Dict[str, Any]:
        """Get a specific blueprint."""
        response = self._request("GET", f"/blueprints/{identifier}")
        return response.json().get("blueprint", {})

    def create_blueprint(self, blueprint: Dict[str, Any]) -> Dict[str, Any]:
        """Create a new blueprint."""
        response = self._request("POST", "/blueprints", data=blueprint)
        return response.json().get("blueprint", {})

    def update_blueprint(
        self, identifier: str, blueprint: Dict[str, Any]
    ) -> Dict[str, Any]:
        """Update an existing blueprint."""
        response = self._request("PUT", f"/blueprints/{identifier}", data=blueprint)
        return response.json().get("blueprint", {})

    def delete_blueprint(self, identifier: str) -> None:
        """Delete a blueprint."""
        self._request("DELETE", f"/blueprints/{identifier}")

    # Entity operations

    def get_entities(
        self, blueprint_identifier: str, params: Optional[Dict[str, Any]] = None
    ) -> List[Dict[str, Any]]:
        """Get entities for a blueprint."""
        response = self._request(
            "GET", f"/blueprints/{blueprint_identifier}/entities", params=params
        )
        return response.json().get("entities", [])

    def get_entity(self, blueprint_identifier: str, entity_identifier: str) -> Dict[str, Any]:
        """Get a specific entity."""
        response = self._request(
            "GET", f"/blueprints/{blueprint_identifier}/entities/{entity_identifier}"
        )
        return response.json().get("entity", {})

    def create_entity(
        self, blueprint_identifier: str, entity: Dict[str, Any]
    ) -> Dict[str, Any]:
        """Create a new entity."""
        response = self._request(
            "POST", f"/blueprints/{blueprint_identifier}/entities", data=entity
        )
        return response.json().get("entity", {})

    def update_entity(
        self, blueprint_identifier: str, entity_identifier: str, entity: Dict[str, Any]
    ) -> Dict[str, Any]:
        """Update an existing entity."""
        response = self._request(
            "PUT",
            f"/blueprints/{blueprint_identifier}/entities/{entity_identifier}",
            data=entity,
        )
        return response.json().get("entity", {})

    def delete_entity(self, blueprint_identifier: str, entity_identifier: str) -> None:
        """Delete an entity."""
        self._request(
            "DELETE", f"/blueprints/{blueprint_identifier}/entities/{entity_identifier}"
        )

    # Scorecard operations

    def get_scorecards(self, blueprint_identifier: str) -> List[Dict[str, Any]]:
        """Get scorecards for a blueprint."""
        response = self._request("GET", f"/blueprints/{blueprint_identifier}/scorecards")
        return response.json().get("scorecards", [])

    def get_all_scorecards(self) -> List[Dict[str, Any]]:
        """Get all scorecards (organization-wide)."""
        response = self._request("GET", "/scorecards")
        return response.json().get("scorecards", [])

    def create_scorecard(self, blueprint_identifier: str, scorecard: Dict[str, Any]) -> Dict[str, Any]:
        """Create a new scorecard for a blueprint."""
        response = self._request("POST", f"/blueprints/{blueprint_identifier}/scorecards", data=scorecard)
        return response.json().get("scorecard", {})

    def update_scorecard(
        self, blueprint_identifier: str, scorecard_identifier: str, scorecard: Dict[str, Any]
    ) -> Dict[str, Any]:
        """Update an existing scorecard."""
        response = self._request(
            "PATCH", f"/blueprints/{blueprint_identifier}/scorecards/{scorecard_identifier}", data=scorecard
        )
        return response.json().get("scorecard", {})

    def delete_scorecard(self, blueprint_identifier: str, scorecard_identifier: str) -> None:
        """Delete a scorecard."""
        self._request("DELETE", f"/blueprints/{blueprint_identifier}/scorecards/{scorecard_identifier}")

    # Action operations

    def get_actions(self, blueprint_identifier: str) -> List[Dict[str, Any]]:
        """Get actions for a blueprint."""
        response = self._request("GET", f"/blueprints/{blueprint_identifier}/actions")
        return response.json().get("actions", [])

    def create_action(self, blueprint_identifier: str, action: Dict[str, Any]) -> Dict[str, Any]:
        """Create a blueprint-level action."""
        response = self._request("POST", f"/blueprints/{blueprint_identifier}/actions", data=action)
        return response.json().get("action", {})

    def update_action(
        self, blueprint_identifier: str, action_identifier: str, action: Dict[str, Any]
    ) -> Dict[str, Any]:
        """Update an existing blueprint-level action."""
        response = self._request(
            "PATCH", f"/blueprints/{blueprint_identifier}/actions/{action_identifier}", data=action
        )
        return response.json().get("action", {})

    def delete_action(self, blueprint_identifier: str, action_identifier: str) -> None:
        """Delete a blueprint-level action."""
        self._request("DELETE", f"/blueprints/{blueprint_identifier}/actions/{action_identifier}")

    # Team operations

    def get_teams(self) -> List[Dict[str, Any]]:
        """Get all teams."""
        response = self._request("GET", "/teams")
        return response.json().get("teams", [])

    def create_team(self, team: Dict[str, Any]) -> Dict[str, Any]:
        """Create a new team."""
        response = self._request("POST", "/teams", data=team)
        return response.json().get("team", {})

    def update_team(self, team_name: str, team: Dict[str, Any]) -> Dict[str, Any]:
        """Update an existing team."""
        response = self._request("PATCH", f"/teams/{team_name}", data=team)
        return response.json().get("team", {})

    def delete_team(self, team_name: str) -> None:
        """Delete a team."""
        self._request("DELETE", f"/teams/{team_name}")

    # Automation operations (combined with actions at org level)

    def get_all_actions(self) -> List[Dict[str, Any]]:
        """Get all actions and automations (organization-wide)."""
        response = self._request("GET", "/actions")
        return response.json().get("actions", [])

    def create_automation(self, automation: Dict[str, Any]) -> Dict[str, Any]:
        """Create a new automation (organization-wide action)."""
        response = self._request("POST", "/actions", data=automation)
        return response.json().get("action", {})

    def update_automation(self, automation_identifier: str, automation: Dict[str, Any]) -> Dict[str, Any]:
        """Update an existing automation."""
        response = self._request("PUT", f"/actions/{automation_identifier}", data=automation)
        return response.json().get("action", {})

    def delete_automation(self, automation_identifier: str) -> None:
        """Delete an automation."""
        self._request("DELETE", f"/actions/{automation_identifier}")

    # Page operations

    def get_pages(self) -> List[Dict[str, Any]]:
        """Get all pages."""
        response = self._request("GET", "/pages")
        return response.json().get("pages", [])

    def create_page(self, page: Dict[str, Any]) -> Dict[str, Any]:
        """Create a new page."""
        response = self._request("POST", "/pages", data=page)
        return response.json().get("page", {})

    def update_page(self, page_identifier: str, page: Dict[str, Any]) -> Dict[str, Any]:
        """Update an existing page."""
        response = self._request("PATCH", f"/pages/{page_identifier}", data=page)
        return response.json().get("page", {})

    def delete_page(self, page_identifier: str) -> None:
        """Delete a page."""
        self._request("DELETE", f"/pages/{page_identifier}")

    # Integration operations

    def get_integrations(self) -> List[Dict[str, Any]]:
        """Get all integrations."""
        response = self._request("GET", "/integration")
        return response.json().get("integrations", [])

    def update_integration_config(self, integration_identifier: str, config: Dict[str, Any]) -> Dict[str, Any]:
        """Update an integration's configuration."""
        response = self._request("PATCH", f"/integration/{integration_identifier}/config", data=config)
        return response.json().get("integration", {})

    def delete_integration(self, integration_identifier: str) -> None:
        """Delete an integration."""
        self._request("DELETE", f"/integration/{integration_identifier}")

