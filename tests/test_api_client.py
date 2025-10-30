"""Tests for Port API client."""

from unittest.mock import Mock, patch

import httpx
import pytest

from port_cli.api_client import PortAPIClient, TokenResponse


@pytest.mark.unit
class TestPortAPIClient:
    """Test Port API client."""

    def test_initialization(self):
        """Test client initialization."""
        client = PortAPIClient(
            client_id="test-id",
            client_secret="test-secret",
            api_url="https://api.getport.io/v1",
        )

        assert client.client_id == "test-id"
        assert client.client_secret == "test-secret"
        assert client.api_url == "https://api.getport.io/v1"
        assert client._token is None

    def test_context_manager(self):
        """Test client as context manager."""
        with PortAPIClient("id", "secret") as client:
            assert client is not None

    @patch("port_cli.api_client.httpx.Client")
    def test_get_token_success(self, mock_httpx):
        """Test successful token retrieval."""
        mock_response = Mock()
        mock_response.status_code = 200
        mock_response.json.return_value = {
            "accessToken": "test-token",
            "expiresIn": 3600,
            "tokenType": "Bearer",
        }

        mock_client = Mock()
        mock_client.post.return_value = mock_response
        mock_httpx.return_value = mock_client

        client = PortAPIClient("id", "secret")
        token = client._get_token()

        assert token == "test-token"
        assert client._token == "test-token"

    @patch("port_cli.api_client.httpx.Client")
    def test_get_token_caching(self, mock_httpx):
        """Test token caching."""
        mock_response = Mock()
        mock_response.status_code = 200
        mock_response.json.return_value = {
            "accessToken": "test-token",
            "expiresIn": 3600,
            "tokenType": "Bearer",
        }

        mock_client = Mock()
        mock_client.post.return_value = mock_response
        mock_httpx.return_value = mock_client

        client = PortAPIClient("id", "secret")

        # First call should fetch token
        token1 = client._get_token()

        # Second call should use cached token
        token2 = client._get_token()

        assert token1 == token2
        assert mock_client.post.call_count == 1  # Only called once

    @patch("port_cli.api_client.httpx.Client")
    def test_get_blueprints(self, mock_httpx):
        """Test getting blueprints."""
        # Mock token request
        mock_token_response = Mock()
        mock_token_response.status_code = 200
        mock_token_response.json.return_value = {
            "accessToken": "test-token",
            "expiresIn": 3600,
            "tokenType": "Bearer",
        }

        # Mock blueprints request
        mock_bp_response = Mock()
        mock_bp_response.status_code = 200
        mock_bp_response.json.return_value = {
            "blueprints": [{"identifier": "service", "title": "Service"}]
        }
        mock_bp_response.raise_for_status = Mock()

        mock_client = Mock()
        mock_client.post.return_value = mock_token_response
        mock_client.request.return_value = mock_bp_response
        mock_httpx.return_value = mock_client

        client = PortAPIClient("id", "secret")
        blueprints = client.get_blueprints()

        assert len(blueprints) == 1
        assert blueprints[0]["identifier"] == "service"

    @patch("port_cli.api_client.httpx.Client")
    def test_create_blueprint(self, mock_httpx):
        """Test creating a blueprint."""
        mock_token_response = Mock()
        mock_token_response.status_code = 200
        mock_token_response.json.return_value = {
            "accessToken": "test-token",
            "expiresIn": 3600,
            "tokenType": "Bearer",
        }

        mock_create_response = Mock()
        mock_create_response.status_code = 201
        mock_create_response.json.return_value = {
            "blueprint": {"identifier": "service", "title": "Service"}
        }
        mock_create_response.raise_for_status = Mock()

        mock_client = Mock()
        mock_client.post.return_value = mock_token_response
        mock_client.request.return_value = mock_create_response
        mock_httpx.return_value = mock_client

        client = PortAPIClient("id", "secret")
        blueprint = client.create_blueprint({"identifier": "service"})

        assert blueprint["identifier"] == "service"

    @patch("port_cli.api_client.httpx.Client")
    def test_get_entities(self, mock_httpx):
        """Test getting entities."""
        mock_token_response = Mock()
        mock_token_response.status_code = 200
        mock_token_response.json.return_value = {
            "accessToken": "test-token",
            "expiresIn": 3600,
            "tokenType": "Bearer",
        }

        mock_entities_response = Mock()
        mock_entities_response.status_code = 200
        mock_entities_response.json.return_value = {
            "entities": [{"identifier": "my-service", "blueprint": "service"}]
        }
        mock_entities_response.raise_for_status = Mock()

        mock_client = Mock()
        mock_client.post.return_value = mock_token_response
        mock_client.request.return_value = mock_entities_response
        mock_httpx.return_value = mock_client

        client = PortAPIClient("id", "secret")
        entities = client.get_entities("service")

        assert len(entities) == 1
        assert entities[0]["identifier"] == "my-service"

    @patch("port_cli.api_client.httpx.Client")
    def test_authentication_error(self, mock_httpx):
        """Test authentication error handling."""
        mock_response = Mock()
        mock_response.status_code = 401
        mock_response.raise_for_status.side_effect = httpx.HTTPStatusError(
            "Unauthorized", request=Mock(), response=mock_response
        )

        mock_client = Mock()
        mock_client.post.return_value = mock_response
        mock_httpx.return_value = mock_client

        client = PortAPIClient("bad-id", "bad-secret")

        with pytest.raises(Exception):
            client._get_token()

