"""Base module interface for all Port CLI modules."""

from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import Any, Dict, Optional


@dataclass
class ModuleResult:
    """Standard result format for all modules."""

    success: bool
    message: str
    data: Optional[Dict[str, Any]] = None
    error: Optional[str] = None


class BaseModule(ABC):
    """
    Base class for all Port CLI modules.
    
    All modules (export, import, migrate, api) must inherit from this class
    and implement the required methods.
    """

    def __init__(self, config: Any):
        """
        Initialize the module with configuration.
        
        Args:
            config: Configuration object (OrganizationConfig or Config)
        """
        self.config = config

    @abstractmethod
    def execute(self, **kwargs: Any) -> ModuleResult:
        """
        Execute the module's main operation.
        
        Args:
            **kwargs: Module-specific arguments
            
        Returns:
            ModuleResult with operation outcome
        """
        pass

    @abstractmethod
    def validate(self, **kwargs: Any) -> bool:
        """
        Validate inputs before execution.
        
        Args:
            **kwargs: Module-specific arguments
            
        Returns:
            True if validation passes, False otherwise
            
        Raises:
            ValueError: If validation fails with specific error
        """
        pass

    def get_name(self) -> str:
        """Get the module name."""
        return self.__class__.__name__

    def get_version(self) -> str:
        """Get the module version."""
        return "1.0.0"

