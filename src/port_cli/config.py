"""Configuration management with file and environment variable support."""

import os
from pathlib import Path
from typing import Any, Dict, Optional

import yaml
from dotenv import load_dotenv
from pydantic import BaseModel, Field


class OrganizationConfig(BaseModel):
    """Configuration for a Port organization."""

    client_id: str
    client_secret: str
    api_url: str = "https://api.getport.io/v1"


class BackendConfig(BaseModel):
    """Configuration for the backend server."""

    url: str = "http://localhost:8080"
    timeout: int = 300


class Config(BaseModel):
    """Main configuration model."""

    default_org: Optional[str] = None
    organizations: Dict[str, OrganizationConfig] = Field(default_factory=dict)
    backend: BackendConfig = Field(default_factory=BackendConfig)


class ConfigManager:
    """Manages configuration loading with precedence: CLI flags > env vars > config file."""

    def __init__(self, config_path: Optional[str] = None):
        """
        Initialize configuration manager.
        
        Automatically loads .env file if present in:
        1. Current directory (.env)
        2. Home directory (~/.port/.env)
        
        Note: .env loading is skipped during testing to prevent interference.
        """
        # Skip .env loading during tests to prevent interference
        is_testing = os.getenv("PYTEST_CURRENT_TEST") is not None
        
        if not is_testing:
            # Load .env file if present (doesn't override existing env vars)
            # Try current directory first
            if Path(".env").exists():
                load_dotenv(".env", override=False)
            # Try ~/.port/.env
            home_env = Path.home() / ".port" / ".env"
            if home_env.exists():
                load_dotenv(home_env, override=False)
        
        if config_path:
            self.config_path = Path(config_path)
        else:
            # Default to ~/.port/config.yaml
            home = Path.home()
            self.config_path = home / ".port" / "config.yaml"

    def load(self) -> Config:
        """
        Load configuration with precedence rules.

        Order of precedence (highest to lowest):
        1. Environment variables (including .env file)
        2. Configuration file (~/.port/config.yaml)
        3. Default values
        
        The .env file is loaded from:
        - Current directory (.env)
        - Home directory (~/.port/.env)
        """
        # Start with defaults
        config_dict: Dict[str, Any] = {
            "default_org": None,
            "organizations": {},
            "backend": {"url": "http://localhost:8080", "timeout": 300},
        }

        # Load from file if exists
        if self.config_path.exists():
            with open(self.config_path, "r") as f:
                file_config = yaml.safe_load(f) or {}
                config_dict.update(file_config)

        # Override with environment variables
        self._load_from_env(config_dict)

        return Config(**config_dict)

    def _load_from_env(self, config_dict: Dict[str, Any]) -> None:
        """Load configuration from environment variables."""
        # Backend URL
        if backend_url := os.getenv("PORT_CLI_BACKEND_URL"):
            config_dict["backend"]["url"] = backend_url

        # Default org from environment
        if default_org := os.getenv("PORT_DEFAULT_ORG"):
            config_dict["default_org"] = default_org

        # Single organization from environment variables
        client_id = os.getenv("PORT_CLIENT_ID")
        client_secret = os.getenv("PORT_CLIENT_SECRET")
        api_url = os.getenv("PORT_API_URL", "https://api.getport.io/v1")

        if client_id and client_secret:
            # Create or override the "default" organization
            org_name = config_dict.get("default_org", "default")
            if "organizations" not in config_dict:
                config_dict["organizations"] = {}

            config_dict["organizations"][org_name] = {
                "client_id": client_id,
                "client_secret": client_secret,
                "api_url": api_url,
            }

            # Set as default org if not set
            if not config_dict.get("default_org"):
                config_dict["default_org"] = org_name

    def create_default_config(self) -> None:
        """Create a default configuration file."""
        # Ensure directory exists
        self.config_path.parent.mkdir(parents=True, exist_ok=True)

        # Create default config
        default_config = {
            "default_org": "production",
            "organizations": {
                "production": {
                    "client_id": "your-client-id",
                    "client_secret": "your-client-secret",
                    "api_url": "https://api.getport.io/v1",
                },
                "staging": {
                    "client_id": "your-staging-client-id",
                    "client_secret": "your-staging-client-secret",
                    "api_url": "https://api.getport.io/v1",
                },
            },
            "backend": {
                "url": "http://localhost:8080",
                "timeout": 300,
            },
        }

        # Write to file
        with open(self.config_path, "w") as f:
            yaml.dump(default_config, f, default_flow_style=False, sort_keys=False)

    def load_with_overrides(
        self,
        client_id: Optional[str] = None,
        client_secret: Optional[str] = None,
        api_url: Optional[str] = None,
        org_name: Optional[str] = None,
    ) -> Config:
        """
        Load configuration with CLI flag overrides.
        
        Precedence (highest to lowest):
        1. CLI flags (passed as arguments)
        2. Environment variables
        3. Configuration file
        4. Default values
        """
        # Load base config (file + env vars)
        config = self.load()
        
        # Apply CLI overrides if provided
        if any([client_id, client_secret, api_url]):
            # Create/update organization with CLI values
            override_org = org_name or config.default_org or "cli-override"
            
            # Get existing org config if it exists
            existing_org = config.organizations.get(override_org)
            
            # Build override config
            override_config = OrganizationConfig(
                client_id=client_id or (existing_org.client_id if existing_org else ""),
                client_secret=client_secret or (existing_org.client_secret if existing_org else ""),
                api_url=api_url or (existing_org.api_url if existing_org else "https://api.getport.io/v1"),
            )
            
            # Validate we have required fields
            if not override_config.client_id or not override_config.client_secret:
                raise ValueError("client_id and client_secret are required when using CLI overrides")
            
            config.organizations[override_org] = override_config
            config.default_org = override_org
        
        return config

    def get_org_config(
        self, config: Config, org_name: Optional[str] = None
    ) -> OrganizationConfig:
        """Get organization configuration by name or use default."""
        org = org_name or config.default_org

        if not org:
            raise ValueError(
                "No organization specified. Use --org flag or set default_org in config."
            )

        if org not in config.organizations:
            raise ValueError(f"Organization '{org}' not found in configuration")

        return config.organizations[org]

