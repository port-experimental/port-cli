"""
Module system for Port CLI.

This package provides a pluggable module architecture where backend modules
(Python or Go) can be dynamically loaded and executed.
"""

from port_cli.modules.base import BaseModule, ModuleResult
from port_cli.modules.registry import ModuleRegistry

__all__ = ["BaseModule", "ModuleResult", "ModuleRegistry"]

