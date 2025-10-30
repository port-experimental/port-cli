"""Module registry for discovering and loading Port CLI modules."""

import importlib
from typing import Any, Dict, Optional, Type

from port_cli.modules.base import BaseModule


class ModuleRegistry:
    """
    Registry for discovering and loading Port CLI modules.
    
    Supports both Python and Go modules with automatic discovery
    and priority-based selection.
    """

    def __init__(self) -> None:
        """Initialize the module registry."""
        self._modules: Dict[str, Type[BaseModule]] = {}
        self._discover_modules()

    def _discover_modules(self) -> None:
        """Discover available modules."""
        # Discover Python modules
        module_names = ["export", "import", "migrate", "api"]
        
        for name in module_names:
            try:
                # Try to import Python implementation
                module_path = f"port_cli.modules.{name}_py"
                module = importlib.import_module(module_path)
                
                # Get the module class (e.g., ExportModule)
                class_name = f"{name.capitalize()}Module"
                module_class = getattr(module, class_name, None)
                
                if module_class and issubclass(module_class, BaseModule):
                    self._modules[name] = module_class
            except (ImportError, AttributeError):
                # Module not available yet
                pass

    def get_module(self, name: str, config: Any) -> Optional[BaseModule]:
        """
        Get a module instance by name.
        
        Args:
            name: Module name (export, import, migrate, api)
            config: Configuration to pass to the module
            
        Returns:
            Instantiated module or None if not found
        """
        module_class = self._modules.get(name)
        if module_class:
            return module_class(config)
        return None

    def list_modules(self) -> list[str]:
        """List all available modules."""
        return list(self._modules.keys())

    def is_available(self, name: str) -> bool:
        """Check if a module is available."""
        return name in self._modules

    def reload(self) -> None:
        """Reload module discovery (useful during development)."""
        self._modules.clear()
        self._discover_modules()

