"""Tests for configuration management."""

import os
import tempfile
from pathlib import Path

import pytest
import yaml

from port_cli.config import Config, ConfigManager, OrganizationConfig


def test_config_model():
    """Test the Config model."""
    config = Config(
        default_org="test",
        organizations={
            "test": OrganizationConfig(
                client_id="id",
                client_secret="secret",
                api_url="https://api.getport.io/v1",
            )
        },
    )

    assert config.default_org == "test"
    assert "test" in config.organizations
    assert config.organizations["test"].client_id == "id"


def test_config_manager_load_from_file():
    """Test loading configuration from file."""
    with tempfile.TemporaryDirectory() as tmpdir:
        config_path = Path(tmpdir) / "config.yaml"

        # Create test config file
        test_config = {
            "default_org": "production",
            "organizations": {
                "production": {
                    "client_id": "test-id",
                    "client_secret": "test-secret",
                    "api_url": "https://api.getport.io/v1",
                }
            },
            "backend": {"url": "http://localhost:8080", "timeout": 300},
        }

        with open(config_path, "w") as f:
            yaml.dump(test_config, f)

        # Load configuration
        manager = ConfigManager(config_path=str(config_path))
        config = manager.load()

        assert config.default_org == "production"
        assert "production" in config.organizations
        assert config.organizations["production"].client_id == "test-id"
        assert config.backend.url == "http://localhost:8080"


def test_config_manager_env_override():
    """Test that environment variables override file configuration."""
    with tempfile.TemporaryDirectory() as tmpdir:
        config_path = Path(tmpdir) / "config.yaml"

        # Create test config file
        test_config = {
            "default_org": "production",
            "organizations": {
                "production": {
                    "client_id": "file-id",
                    "client_secret": "file-secret",
                    "api_url": "https://api.getport.io/v1",
                }
            },
        }

        with open(config_path, "w") as f:
            yaml.dump(test_config, f)

        # Set environment variables
        os.environ["PORT_CLIENT_ID"] = "env-id"
        os.environ["PORT_CLIENT_SECRET"] = "env-secret"
        os.environ["PORT_CLI_BACKEND_URL"] = "http://backend:9000"

        try:
            manager = ConfigManager(config_path=str(config_path))
            config = manager.load()

            # Env vars should create/override the default org
            assert config.organizations["production"].client_id == "env-id"
            assert config.organizations["production"].client_secret == "env-secret"
            assert config.backend.url == "http://backend:9000"
        finally:
            # Cleanup
            os.environ.pop("PORT_CLIENT_ID", None)
            os.environ.pop("PORT_CLIENT_SECRET", None)
            os.environ.pop("PORT_CLI_BACKEND_URL", None)


def test_config_manager_get_org_config():
    """Test getting organization configuration."""
    with tempfile.TemporaryDirectory() as tmpdir:
        config_path = Path(tmpdir) / "config.yaml"

        test_config = {
            "default_org": "production",
            "organizations": {
                "production": {
                    "client_id": "prod-id",
                    "client_secret": "prod-secret",
                    "api_url": "https://api.getport.io/v1",
                },
                "staging": {
                    "client_id": "staging-id",
                    "client_secret": "staging-secret",
                    "api_url": "https://api.getport.io/v1",
                },
            },
        }

        with open(config_path, "w") as f:
            yaml.dump(test_config, f)

        manager = ConfigManager(config_path=str(config_path))
        config = manager.load()

        # Get default org
        org_config = manager.get_org_config(config)
        assert org_config.client_id == "prod-id"

        # Get specific org
        org_config = manager.get_org_config(config, "staging")
        assert org_config.client_id == "staging-id"

        # Non-existent org should raise error
        with pytest.raises(ValueError, match="not found"):
            manager.get_org_config(config, "non-existent")

