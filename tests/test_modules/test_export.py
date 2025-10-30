"""Tests for export module."""

import json
import tarfile
from pathlib import Path
from unittest.mock import Mock, patch

import pytest

from port_cli.modules.export_py import ExportModule


@pytest.mark.unit
class TestExportModule:
    """Test export module functionality."""

    def test_initialization(self, mock_org_config):
        """Test export module initialization."""
        module = ExportModule(mock_org_config)
        assert module.config == mock_org_config

    def test_validate_success(self, mock_org_config):
        """Test successful validation."""
        module = ExportModule(mock_org_config)
        assert module.validate(output_path="backup.tar.gz", format="tar")

    def test_validate_missing_output_path(self, mock_org_config):
        """Test validation fails without output path."""
        module = ExportModule(mock_org_config)

        with pytest.raises(ValueError, match="output_path is required"):
            module.validate()

    def test_validate_invalid_format(self, mock_org_config):
        """Test validation fails with invalid format."""
        module = ExportModule(mock_org_config)

        with pytest.raises(ValueError, match="format must be"):
            module.validate(output_path="backup.txt", format="invalid")

    @patch("port_cli.modules.export_py.exporter.PortAPIClient")
    def test_execute_json_format(
        self, mock_api_class, mock_org_config, tmp_path, sample_export_data
    ):
        """Test executing export with JSON format."""
        # Setup mock API client
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
        mock_api_class.return_value = mock_client

        module = ExportModule(mock_org_config)
        output_file = tmp_path / "backup.json"

        result = module.execute(output_path=str(output_file), format="json")

        assert result.success
        assert "Successfully exported" in result.message
        assert output_file.exists()

        # Verify exported data
        with open(output_file) as f:
            data = json.load(f)
            assert "blueprints" in data
            assert "entities" in data

    @patch("port_cli.modules.export_py.exporter.PortAPIClient")
    def test_execute_tar_format(
        self, mock_api_class, mock_org_config, tmp_path, sample_export_data
    ):
        """Test executing export with tar.gz format."""
        # Setup mock API client
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
        mock_api_class.return_value = mock_client

        module = ExportModule(mock_org_config)
        output_file = tmp_path / "backup.tar.gz"

        result = module.execute(output_path=str(output_file), format="tar")

        assert result.success
        assert output_file.exists()

        # Verify tar file contents
        with tarfile.open(output_file, "r:gz") as tar:
            members = tar.getnames()
            assert "blueprints.json" in members
            assert "entities.json" in members

    @patch("port_cli.modules.export_py.exporter.PortAPIClient")
    def test_execute_selective_blueprints(
        self, mock_api_class, mock_org_config, tmp_path, sample_export_data
    ):
        """Test executing export with selective blueprints."""
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
        mock_api_class.return_value = mock_client

        module = ExportModule(mock_org_config)
        output_file = tmp_path / "backup.json"

        result = module.execute(
            output_path=str(output_file), blueprints=["service"], format="json"
        )

        assert result.success
        assert result.data["blueprints_count"] == 1

    @patch("port_cli.modules.export_py.exporter.PortAPIClient")
    def test_execute_handles_errors(self, mock_api_class, mock_org_config, tmp_path):
        """Test export handles errors gracefully."""
        mock_client = Mock()
        mock_client.__enter__ = Mock(return_value=mock_client)
        mock_client.__exit__ = Mock(return_value=None)
        mock_client.get_blueprints.side_effect = Exception("API Error")
        mock_api_class.return_value = mock_client

        module = ExportModule(mock_org_config)
        output_file = tmp_path / "backup.json"

        result = module.execute(output_path=str(output_file), format="json")

        assert not result.success
        assert "API Error" in result.error

