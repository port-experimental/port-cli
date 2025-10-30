"""Tests for module registry."""

import pytest

from port_cli.config import OrganizationConfig
from port_cli.modules.registry import ModuleRegistry


@pytest.mark.unit
class TestModuleRegistry:
    """Test module registry functionality."""

    def test_initialization(self):
        """Test registry initialization."""
        registry = ModuleRegistry()
        assert registry is not None
        assert isinstance(registry._modules, dict)

    def test_list_modules(self):
        """Test listing available modules."""
        registry = ModuleRegistry()
        modules = registry.list_modules()

        assert isinstance(modules, list)
        # Check that core modules are discovered
        assert "export" in modules
        assert "import" in modules
        assert "migrate" in modules

    def test_is_available(self):
        """Test checking module availability."""
        registry = ModuleRegistry()

        assert registry.is_available("export")
        assert registry.is_available("import")
        assert registry.is_available("migrate")
        assert not registry.is_available("nonexistent")

    def test_get_module(self, mock_org_config: OrganizationConfig):
        """Test getting a module instance."""
        registry = ModuleRegistry()
        module = registry.get_module("export", mock_org_config)

        assert module is not None
        assert module.config == mock_org_config

    def test_get_nonexistent_module(self, mock_org_config: OrganizationConfig):
        """Test getting a nonexistent module returns None."""
        registry = ModuleRegistry()
        module = registry.get_module("nonexistent", mock_org_config)

        assert module is None

    def test_reload(self):
        """Test reloading module discovery."""
        registry = ModuleRegistry()
        initial_modules = registry.list_modules()

        registry.reload()
        reloaded_modules = registry.list_modules()

        assert initial_modules == reloaded_modules

