"""Tests for base module functionality."""

import pytest

from port_cli.modules.base import BaseModule, ModuleResult


@pytest.mark.unit
class TestModuleResult:
    """Test ModuleResult dataclass."""

    def test_success_result(self):
        """Test creating a successful result."""
        result = ModuleResult(
            success=True,
            message="Operation completed",
            data={"count": 5},
        )

        assert result.success is True
        assert result.message == "Operation completed"
        assert result.data == {"count": 5}
        assert result.error is None

    def test_failure_result(self):
        """Test creating a failure result."""
        result = ModuleResult(
            success=False,
            message="Operation failed",
            error="Something went wrong",
        )

        assert result.success is False
        assert result.message == "Operation failed"
        assert result.error == "Something went wrong"
        assert result.data is None


@pytest.mark.unit
class TestBaseModule:
    """Test BaseModule abstract class."""

    def test_cannot_instantiate_directly(self, mock_org_config):
        """Test that BaseModule cannot be instantiated directly."""
        with pytest.raises(TypeError):
            BaseModule(mock_org_config)

    def test_concrete_implementation(self, mock_org_config):
        """Test that concrete implementations work."""

        class ConcreteModule(BaseModule):
            def validate(self, **kwargs):
                return True

            def execute(self, **kwargs):
                return ModuleResult(success=True, message="Done")

        module = ConcreteModule(mock_org_config)
        assert module.config == mock_org_config
        assert module.validate() is True

        result = module.execute()
        assert result.success is True
        assert result.message == "Done"

    def test_get_name(self, mock_org_config):
        """Test getting module name."""

        class TestModule(BaseModule):
            def validate(self, **kwargs):
                return True

            def execute(self, **kwargs):
                return ModuleResult(success=True, message="Done")

        module = TestModule(mock_org_config)
        assert module.get_name() == "TestModule"

    def test_get_version(self, mock_org_config):
        """Test getting module version."""

        class TestModule(BaseModule):
            def validate(self, **kwargs):
                return True

            def execute(self, **kwargs):
                return ModuleResult(success=True, message="Done")

        module = TestModule(mock_org_config)
        version = module.get_version()
        assert isinstance(version, str)
        assert len(version) > 0

