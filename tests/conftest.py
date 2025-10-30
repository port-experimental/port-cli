"""Shared pytest fixtures and configuration."""

import json
import tempfile
from pathlib import Path
from typing import Any, Dict
from unittest.mock import Mock

import pytest
import yaml

from port_cli.config import Config, OrganizationConfig


@pytest.fixture
def mock_org_config() -> OrganizationConfig:
    """Create a mock organization configuration."""
    return OrganizationConfig(
        client_id="test-client-id",
        client_secret="test-client-secret",
        api_url="https://api.getport.io/v1",
    )


@pytest.fixture
def mock_config(mock_org_config: OrganizationConfig) -> Config:
    """Create a mock full configuration."""
    return Config(
        default_org="test",
        organizations={"test": mock_org_config},
    )


@pytest.fixture
def temp_config_file(tmp_path: Path, mock_org_config: OrganizationConfig) -> Path:
    """Create a temporary configuration file."""
    config_dir = tmp_path / ".port"
    config_dir.mkdir()
    config_file = config_dir / "config.yaml"

    config_data = {
        "default_org": "test",
        "organizations": {
            "test": {
                "client_id": mock_org_config.client_id,
                "client_secret": mock_org_config.client_secret,
                "api_url": mock_org_config.api_url,
            }
        },
    }

    with open(config_file, "w") as f:
        yaml.dump(config_data, f)

    return config_file


@pytest.fixture
def sample_blueprint() -> Dict[str, Any]:
    """Create a sample blueprint."""
    return {
        "identifier": "service",
        "title": "Service",
        "description": "A microservice",
        "schema": {
            "properties": {
                "name": {"type": "string"},
                "language": {"type": "string"},
            }
        },
    }


@pytest.fixture
def sample_entity() -> Dict[str, Any]:
    """Create a sample entity."""
    return {
        "identifier": "my-service",
        "title": "My Service",
        "blueprint": "service",
        "properties": {
            "name": "My Service",
            "language": "Python",
        },
    }


@pytest.fixture
def sample_export_data(
    sample_blueprint: Dict[str, Any], sample_entity: Dict[str, Any]
) -> Dict[str, Any]:
    """Create sample export data."""
    return {
        "blueprints": [sample_blueprint],
        "entities": [sample_entity],
        "scorecards": [],
        "actions": [],
        "teams": [],
        "automations": [],
        "pages": [],
        "integrations": [],
    }


@pytest.fixture
def temp_export_file(tmp_path: Path, sample_export_data: Dict[str, Any]) -> Path:
    """Create a temporary export JSON file."""
    export_file = tmp_path / "export.json"
    with open(export_file, "w") as f:
        json.dump(sample_export_data, f)
    return export_file


@pytest.fixture
def mock_port_api_client(
    sample_blueprint: Dict[str, Any], sample_entity: Dict[str, Any]
) -> Mock:
    """Create a mock Port API client."""
    mock_client = Mock()

    # Mock token generation
    mock_client._get_token.return_value = "mock-token"

    # Mock blueprint operations
    mock_client.get_blueprints.return_value = [sample_blueprint]
    mock_client.get_blueprint.return_value = sample_blueprint
    mock_client.create_blueprint.return_value = sample_blueprint
    mock_client.update_blueprint.return_value = sample_blueprint
    mock_client.delete_blueprint.return_value = None

    # Mock entity operations
    mock_client.get_entities.return_value = [sample_entity]
    mock_client.get_entity.return_value = sample_entity
    mock_client.create_entity.return_value = sample_entity
    mock_client.update_entity.return_value = sample_entity
    mock_client.delete_entity.return_value = None

    # Mock other operations
    mock_client.get_scorecards.return_value = []
    mock_client.get_actions.return_value = []
    mock_client.get_teams.return_value = []
    mock_client.get_all_actions.return_value = []
    mock_client.get_all_scorecards.return_value = []
    mock_client.get_pages.return_value = []
    mock_client.get_integrations.return_value = []

    # Mock scorecard operations
    mock_client.create_scorecard.return_value = {"identifier": "test-scorecard"}
    mock_client.update_scorecard.return_value = {"identifier": "test-scorecard"}
    mock_client.delete_scorecard.return_value = None

    # Mock action operations
    mock_client.create_action.return_value = {"identifier": "test-action"}
    mock_client.update_action.return_value = {"identifier": "test-action"}
    mock_client.delete_action.return_value = None

    # Mock team operations
    mock_client.create_team.return_value = {"name": "test-team"}
    mock_client.update_team.return_value = {"name": "test-team"}
    mock_client.delete_team.return_value = None

    # Mock automation operations
    mock_client.create_automation.return_value = {"identifier": "test-automation"}
    mock_client.update_automation.return_value = {"identifier": "test-automation"}
    mock_client.delete_automation.return_value = None

    # Mock page operations
    mock_client.create_page.return_value = {"identifier": "test-page"}
    mock_client.update_page.return_value = {"identifier": "test-page"}
    mock_client.delete_page.return_value = None

    # Mock integration operations
    mock_client.update_integration_config.return_value = {"identifier": "test-integration"}
    mock_client.delete_integration.return_value = None

    # Context manager support
    mock_client.__enter__ = Mock(return_value=mock_client)
    mock_client.__exit__ = Mock(return_value=None)

    return mock_client


@pytest.fixture
def temp_dir(tmp_path: Path) -> Path:
    """Create a temporary directory for test files."""
    return tmp_path


@pytest.fixture(autouse=True)
def clean_env(monkeypatch: pytest.MonkeyPatch) -> None:
    """Clean environment variables before each test."""
    env_vars = [
        "PORT_CLIENT_ID",
        "PORT_CLIENT_SECRET",
        "PORT_API_URL",
        "PORT_CONFIG_FILE",
        "PORT_DEFAULT_ORG",
        "PORT_DEBUG",
    ]
    for var in env_vars:
        monkeypatch.delenv(var, raising=False)

