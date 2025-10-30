"""Tests for migrate module."""

from unittest.mock import Mock, patch

import pytest

from port_cli.modules.migrate_py import MigrateModule


@pytest.mark.unit
class TestMigrateModule:
    """Test migrate module functionality."""

    def test_initialization(self, mock_org_config):
        """Test migrate module initialization."""
        source_config = mock_org_config
        target_config = mock_org_config

        module = MigrateModule(source_config, target_config)

        assert module.source_config == source_config
        assert module.target_config == target_config

    def test_validate_success(self, mock_org_config):
        """Test successful validation."""
        module = MigrateModule(mock_org_config, mock_org_config)
        assert module.validate()

    def test_validate_missing_configs(self):
        """Test validation fails without configs."""
        module = MigrateModule(None, None)

        with pytest.raises(ValueError, match="source and target"):
            module.validate()

    @patch("port_cli.modules.migrate_py.migrator.PortAPIClient")
    def test_execute_dry_run(self, mock_api_class, mock_org_config, sample_export_data):
        """Test executing migration in dry run mode."""
        mock_client = Mock()
        mock_client.__enter__ = Mock(return_value=mock_client)
        mock_client.__exit__ = Mock(return_value=None)
        mock_client.get_blueprints.return_value = sample_export_data["blueprints"]
        mock_client.get_entities.return_value = sample_export_data["entities"]
        mock_client.get_scorecards.return_value = []
        mock_client.get_actions.return_value = []
        mock_client.get_teams.return_value = []
        mock_api_class.return_value = mock_client

        module = MigrateModule(mock_org_config, mock_org_config)

        result = module.execute(dry_run=True)

        assert result.success
        assert "dry run" in result.message.lower()

    @patch("port_cli.modules.migrate_py.migrator.PortAPIClient")
    def test_execute_success(self, mock_api_class, mock_org_config, sample_export_data):
        """Test successful migration execution."""
        mock_client = Mock()
        mock_client.__enter__ = Mock(return_value=mock_client)
        mock_client.__exit__ = Mock(return_value=None)
        mock_client.get_blueprints.return_value = sample_export_data["blueprints"]
        mock_client.get_entities.return_value = sample_export_data["entities"]
        mock_client.get_scorecards.return_value = []
        mock_client.get_actions.return_value = []
        mock_client.get_teams.return_value = []
        mock_client.get_all_actions.return_value = []
        mock_client.get_pages.return_value = []
        mock_client.get_integrations.return_value = []
        mock_client.create_blueprint.return_value = sample_export_data["blueprints"][0]
        mock_client.get_entity.side_effect = Exception("Not found")
        mock_client.create_entity.return_value = sample_export_data["entities"][0]
        mock_api_class.return_value = mock_client

        module = MigrateModule(mock_org_config, mock_org_config)

        result = module.execute(dry_run=False)

        assert result.success
        assert "blueprints_created" in result.data

    @patch("port_cli.modules.migrate_py.migrator.PortAPIClient")
    def test_execute_selective_blueprints(
        self, mock_api_class, mock_org_config, sample_export_data
    ):
        """Test migration with selective blueprints."""
        mock_client = Mock()
        mock_client.__enter__ = Mock(return_value=mock_client)
        mock_client.__exit__ = Mock(return_value=None)
        mock_client.get_blueprints.return_value = sample_export_data["blueprints"]
        mock_client.get_entities.return_value = sample_export_data["entities"]
        mock_client.get_scorecards.return_value = []
        mock_client.get_actions.return_value = []
        mock_client.get_teams.return_value = []
        mock_client.get_all_actions.return_value = []
        mock_client.get_pages.return_value = []
        mock_client.get_integrations.return_value = []
        mock_client.create_blueprint.return_value = sample_export_data["blueprints"][0]
        mock_api_class.return_value = mock_client

        module = MigrateModule(mock_org_config, mock_org_config)

        result = module.execute(blueprints=["service"], dry_run=False)

        assert result.success

    @patch("port_cli.modules.migrate_py.migrator.PortAPIClient")
    def test_execute_handles_errors(self, mock_api_class, mock_org_config):
        """Test migration handles errors gracefully."""
        mock_client = Mock()
        mock_client.__enter__ = Mock(return_value=mock_client)
        mock_client.__exit__ = Mock(return_value=None)
        mock_client.get_blueprints.side_effect = Exception("API Error")
        mock_api_class.return_value = mock_client

        module = MigrateModule(mock_org_config, mock_org_config)

        result = module.execute(dry_run=False)

        assert not result.success
        assert "API Error" in result.error

    def test_dependency_resolution(self, mock_org_config, sample_blueprint):
        """Test blueprint dependency resolution."""
        # Create blueprints with dependencies
        blueprint1 = {
            "identifier": "service",
            "title": "Service",
            "relations": {},
        }
        blueprint2 = {
            "identifier": "deployment",
            "title": "Deployment",
            "relations": {
                "service": {"target": "service"}
            },
        }

        all_blueprints = [blueprint1, blueprint2]
        selected = [blueprint2]  # Only select deployment

        module = MigrateModule(mock_org_config, mock_org_config)
        resolved = module._resolve_dependencies(all_blueprints, selected)

        # Should include both deployment and its dependency (service)
        identifiers = [bp["identifier"] for bp in resolved]
        assert "deployment" in identifiers
        assert "service" in identifiers

