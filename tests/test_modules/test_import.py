"""Tests for import module."""

from unittest.mock import Mock, patch

import pytest

from port_cli.modules.import_py import ImportModule


@pytest.mark.unit
class TestImportModule:
    """Test import module functionality."""

    def test_initialization(self, mock_org_config):
        """Test import module initialization."""
        module = ImportModule(mock_org_config)
        assert module.config == mock_org_config

    def test_validate_success(self, mock_org_config, temp_export_file):
        """Test successful validation."""
        module = ImportModule(mock_org_config)
        assert module.validate(input_path=str(temp_export_file))

    def test_validate_missing_input_path(self, mock_org_config):
        """Test validation fails without input path."""
        module = ImportModule(mock_org_config)

        with pytest.raises(ValueError, match="input_path is required"):
            module.validate()

    def test_validate_nonexistent_file(self, mock_org_config):
        """Test validation fails with nonexistent file."""
        module = ImportModule(mock_org_config)

        with pytest.raises(ValueError, match="does not exist"):
            module.validate(input_path="nonexistent.json")

    @patch("port_cli.modules.import_py.importer.PortAPIClient")
    def test_execute_dry_run(
        self, mock_api_class, mock_org_config, temp_export_file
    ):
        """Test executing import in dry run mode."""
        module = ImportModule(mock_org_config)

        result = module.execute(input_path=str(temp_export_file), dry_run=True)

        assert result.success
        assert "dry run" in result.message.lower()
        assert "no changes applied" in result.message.lower()

    @patch("port_cli.modules.import_py.importer.PortAPIClient")
    def test_execute_success(
        self, mock_api_class, mock_org_config, temp_export_file, sample_export_data
    ):
        """Test successful import execution."""
        mock_client = Mock()
        mock_client.__enter__ = Mock(return_value=mock_client)
        mock_client.__exit__ = Mock(return_value=None)
        mock_client.get_blueprints.return_value = []
        mock_client.create_blueprint.return_value = sample_export_data["blueprints"][0]
        mock_client.get_entity.side_effect = Exception("Not found")
        mock_client.create_entity.return_value = sample_export_data["entities"][0]
        mock_api_class.return_value = mock_client

        module = ImportModule(mock_org_config)

        result = module.execute(input_path=str(temp_export_file), dry_run=False)

        assert result.success
        assert result.data["blueprints_created"] >= 0

    @patch("port_cli.modules.import_py.importer.PortAPIClient")
    def test_execute_skip_existing(
        self, mock_api_class, mock_org_config, temp_export_file, sample_export_data
    ):
        """Test import skips existing resources."""
        mock_client = Mock()
        mock_client.__enter__ = Mock(return_value=mock_client)
        mock_client.__exit__ = Mock(return_value=None)
        # Simulate existing blueprints
        mock_client.get_blueprints.return_value = sample_export_data["blueprints"]
        mock_api_class.return_value = mock_client

        module = ImportModule(mock_org_config)

        result = module.execute(
            input_path=str(temp_export_file),
            dry_run=False,
            conflict_handler="skip",
        )

        assert result.success
        assert result.data["blueprints_created"] == 0

    @patch("port_cli.modules.import_py.importer.PortAPIClient")
    def test_execute_overwrite_existing(
        self, mock_api_class, mock_org_config, temp_export_file, sample_export_data
    ):
        """Test import overwrites existing resources."""
        mock_client = Mock()
        mock_client.__enter__ = Mock(return_value=mock_client)
        mock_client.__exit__ = Mock(return_value=None)
        mock_client.get_blueprints.return_value = sample_export_data["blueprints"]
        mock_client.update_blueprint.return_value = sample_export_data["blueprints"][0]
        mock_api_class.return_value = mock_client

        module = ImportModule(mock_org_config)

        result = module.execute(
            input_path=str(temp_export_file),
            dry_run=False,
            conflict_handler="overwrite",
        )

        assert result.success
        assert result.data["blueprints_updated"] >= 0

    @patch("port_cli.modules.import_py.importer.PortAPIClient")
    def test_execute_handles_errors(
        self, mock_api_class, mock_org_config, temp_export_file
    ):
        """Test import handles errors gracefully."""
        mock_client = Mock()
        mock_client.__enter__ = Mock(return_value=mock_client)
        mock_client.__exit__ = Mock(return_value=None)
        mock_client.get_blueprints.side_effect = Exception("API Error")
        mock_api_class.return_value = mock_client

        module = ImportModule(mock_org_config)

        result = module.execute(input_path=str(temp_export_file), dry_run=False)

        assert not result.success
        assert "API Error" in result.error

