"""Direct Port API operation commands."""

import json
from pathlib import Path
from typing import Any, Dict, Optional

import typer
from rich.console import Console
from rich.table import Table

from port_cli.api_client import PortAPIClient
from port_cli.config import ConfigManager

console = Console()
app = typer.Typer(help="Direct Port API operations")

# Subcommands for different resources
blueprints_app = typer.Typer(help="Blueprint operations")
entities_app = typer.Typer(help="Entity operations")

app.add_typer(blueprints_app, name="blueprints")
app.add_typer(entities_app, name="entities")


def format_output(data: Any, format: str = "json") -> None:
    """Format and display output."""
    if format == "json":
        console.print_json(json.dumps(data))
    elif format == "yaml":
        # Simple YAML-like output
        console.print(data)
    else:
        console.print(data)


# Blueprint commands


@blueprints_app.command("list")
def list_blueprints(
    ctx: typer.Context,
    org: Optional[str] = typer.Option(
        None, "--org", help="Organization name (uses default if not specified)"
    ),
    format: str = typer.Option("json", "--format", "-f", help="Output format: json, yaml, table"),
) -> None:
    """List all blueprints."""
    try:
        # Get CLI overrides from context
        cli_overrides = ctx.obj or {}
        
        # Load configuration with CLI overrides
        config_manager = ConfigManager(cli_overrides.get("config_file"))
        config = config_manager.load_with_overrides(
            client_id=cli_overrides.get("client_id"),
            client_secret=cli_overrides.get("client_secret"),
            api_url=cli_overrides.get("api_url"),
            org_name=org,
        )
        org_config = config_manager.get_org_config(config, org)

        with PortAPIClient(
            client_id=org_config.client_id,
            client_secret=org_config.client_secret,
            api_url=org_config.api_url
        ) as client:
            result = client.get_blueprints()
            format_output(result, format)

    except Exception as e:
        console.print(f"[red]✗[/red] Failed to list blueprints: {e}")
        raise typer.Exit(code=1)


@blueprints_app.command("get")
def get_blueprint(
    ctx: typer.Context,
    blueprint_id: str = typer.Argument(..., help="Blueprint ID"),
    org: Optional[str] = typer.Option(
        None, "--org", help="Organization name (uses default if not specified)"
    ),
    format: str = typer.Option("json", "--format", "-f", help="Output format: json, yaml"),
) -> None:
    """Get a specific blueprint."""
    try:
        # Get CLI overrides from context
        cli_overrides = ctx.obj or {}
        
        # Load configuration with CLI overrides
        config_manager = ConfigManager(cli_overrides.get("config_file"))
        config = config_manager.load_with_overrides(
            client_id=cli_overrides.get("client_id"),
            client_secret=cli_overrides.get("client_secret"),
            api_url=cli_overrides.get("api_url"),
            org_name=org,
        )
        org_config = config_manager.get_org_config(config, org)

        with PortAPIClient(
            client_id=org_config.client_id,
            client_secret=org_config.client_secret,
            api_url=org_config.api_url
        ) as client:
            result = client.get_blueprint(blueprint_id)
            format_output(result, format)

    except Exception as e:
        console.print(f"[red]✗[/red] Failed to get blueprint: {e}")
        raise typer.Exit(code=1)


@blueprints_app.command("create")
def create_blueprint(
    ctx: typer.Context,
    data_file: Path = typer.Option(..., "--data", "-d", help="JSON file with blueprint data"),
    org: Optional[str] = typer.Option(
        None, "--org", help="Organization name (uses default if not specified)"
    ),
) -> None:
    """Create a new blueprint."""
    try:
        if not data_file.exists():
            console.print(f"[red]✗[/red] Data file not found: {data_file}")
            raise typer.Exit(code=1)

        with open(data_file, "r") as f:
            data = json.load(f)

        # Get CLI overrides from context
        cli_overrides = ctx.obj or {}
        
        # Load configuration with CLI overrides
        config_manager = ConfigManager(cli_overrides.get("config_file"))
        config = config_manager.load_with_overrides(
            client_id=cli_overrides.get("client_id"),
            client_secret=cli_overrides.get("client_secret"),
            api_url=cli_overrides.get("api_url"),
            org_name=org,
        )
        org_config = config_manager.get_org_config(config, org)

        with PortAPIClient(
            client_id=org_config.client_id,
            client_secret=org_config.client_secret,
            api_url=org_config.api_url
        ) as client:
            result = client.create_blueprint(data)
            console.print("[green]✓[/green] Blueprint created successfully!")
            format_output(result, "json")

    except Exception as e:
        console.print(f"[red]✗[/red] Failed to create blueprint: {e}")
        raise typer.Exit(code=1)


@blueprints_app.command("delete")
def delete_blueprint(
    ctx: typer.Context,
    blueprint_id: str = typer.Argument(..., help="Blueprint ID"),
    org: Optional[str] = typer.Option(
        None, "--org", help="Organization name (uses default if not specified)"
    ),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
) -> None:
    """Delete a blueprint."""
    try:
        if not force:
            confirm = typer.confirm(
                f"Are you sure you want to delete blueprint '{blueprint_id}'?"
            )
            if not confirm:
                console.print("[yellow]Operation cancelled[/yellow]")
                raise typer.Exit(0)

        # Get CLI overrides from context
        cli_overrides = ctx.obj or {}
        
        # Load configuration with CLI overrides
        config_manager = ConfigManager(cli_overrides.get("config_file"))
        config = config_manager.load_with_overrides(
            client_id=cli_overrides.get("client_id"),
            client_secret=cli_overrides.get("client_secret"),
            api_url=cli_overrides.get("api_url"),
            org_name=org,
        )
        org_config = config_manager.get_org_config(config, org)

        with PortAPIClient(
            client_id=org_config.client_id,
            client_secret=org_config.client_secret,
            api_url=org_config.api_url
        ) as client:
            client.delete_blueprint(blueprint_id)
            console.print(f"[green]✓[/green] Blueprint '{blueprint_id}' deleted successfully!")

    except Exception as e:
        console.print(f"[red]✗[/red] Failed to delete blueprint: {e}")
        raise typer.Exit(code=1)


# Entity commands


@entities_app.command("list")
def list_entities(
    ctx: typer.Context,
    blueprint: Optional[str] = typer.Option(
        None, "--blueprint", "-b", help="Filter by blueprint ID"
    ),
    org: Optional[str] = typer.Option(
        None, "--org", help="Organization name (uses default if not specified)"
    ),
    format: str = typer.Option("json", "--format", "-f", help="Output format: json, yaml, table"),
) -> None:
    """List entities."""
    try:
        # Get CLI overrides from context
        cli_overrides = ctx.obj or {}
        
        # Load configuration with CLI overrides
        config_manager = ConfigManager(cli_overrides.get("config_file"))
        config = config_manager.load_with_overrides(
            client_id=cli_overrides.get("client_id"),
            client_secret=cli_overrides.get("client_secret"),
            api_url=cli_overrides.get("api_url"),
            org_name=org,
        )
        org_config = config_manager.get_org_config(config, org)

        with PortAPIClient(
            client_id=org_config.client_id,
            client_secret=org_config.client_secret,
            api_url=org_config.api_url
        ) as client:
            if blueprint:
                result = client.get_entities(blueprint)
            else:
                # Get all blueprints and then all entities
                blueprints = client.get_blueprints()
                result = []
                for bp in blueprints:
                    entities = client.get_entities(bp["identifier"])
                    result.extend(entities)
            format_output(result, format)

    except Exception as e:
        console.print(f"[red]✗[/red] Failed to list entities: {e}")
        raise typer.Exit(code=1)


@entities_app.command("get")
def get_entity(
    ctx: typer.Context,
    blueprint_id: str = typer.Argument(..., help="Blueprint ID"),
    entity_id: str = typer.Argument(..., help="Entity ID"),
    org: Optional[str] = typer.Option(
        None, "--org", help="Organization name (uses default if not specified)"
    ),
    format: str = typer.Option("json", "--format", "-f", help="Output format: json, yaml"),
) -> None:
    """Get a specific entity."""
    try:
        # Get CLI overrides from context
        cli_overrides = ctx.obj or {}
        
        # Load configuration with CLI overrides
        config_manager = ConfigManager(cli_overrides.get("config_file"))
        config = config_manager.load_with_overrides(
            client_id=cli_overrides.get("client_id"),
            client_secret=cli_overrides.get("client_secret"),
            api_url=cli_overrides.get("api_url"),
            org_name=org,
        )
        org_config = config_manager.get_org_config(config, org)

        with PortAPIClient(
            client_id=org_config.client_id,
            client_secret=org_config.client_secret,
            api_url=org_config.api_url
        ) as client:
            result = client.get_entity(blueprint_id, entity_id)
            format_output(result, format)

    except Exception as e:
        console.print(f"[red]✗[/red] Failed to get entity: {e}")
        raise typer.Exit(code=1)


@entities_app.command("create")
def create_entity(
    ctx: typer.Context,
    blueprint_id: str = typer.Argument(..., help="Blueprint ID"),
    data_file: Path = typer.Option(..., "--data", "-d", help="JSON file with entity data"),
    org: Optional[str] = typer.Option(
        None, "--org", help="Organization name (uses default if not specified)"
    ),
) -> None:
    """Create a new entity."""
    try:
        if not data_file.exists():
            console.print(f"[red]✗[/red] Data file not found: {data_file}")
            raise typer.Exit(code=1)

        with open(data_file, "r") as f:
            data = json.load(f)

        # Get CLI overrides from context
        cli_overrides = ctx.obj or {}
        
        # Load configuration with CLI overrides
        config_manager = ConfigManager(cli_overrides.get("config_file"))
        config = config_manager.load_with_overrides(
            client_id=cli_overrides.get("client_id"),
            client_secret=cli_overrides.get("client_secret"),
            api_url=cli_overrides.get("api_url"),
            org_name=org,
        )
        org_config = config_manager.get_org_config(config, org)

        with PortAPIClient(
            client_id=org_config.client_id,
            client_secret=org_config.client_secret,
            api_url=org_config.api_url
        ) as client:
            result = client.create_entity(blueprint_id, data)
            console.print("[green]✓[/green] Entity created successfully!")
            format_output(result, "json")

    except Exception as e:
        console.print(f"[red]✗[/red] Failed to create entity: {e}")
        raise typer.Exit(code=1)


@entities_app.command("delete")
def delete_entity(
    ctx: typer.Context,
    blueprint_id: str = typer.Argument(..., help="Blueprint ID"),
    entity_id: str = typer.Argument(..., help="Entity ID"),
    org: Optional[str] = typer.Option(
        None, "--org", help="Organization name (uses default if not specified)"
    ),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
) -> None:
    """Delete an entity."""
    try:
        if not force:
            confirm = typer.confirm(
                f"Are you sure you want to delete entity '{entity_id}' from blueprint '{blueprint_id}'?"
            )
            if not confirm:
                console.print("[yellow]Operation cancelled[/yellow]")
                raise typer.Exit(0)

        # Get CLI overrides from context
        cli_overrides = ctx.obj or {}
        
        # Load configuration with CLI overrides
        config_manager = ConfigManager(cli_overrides.get("config_file"))
        config = config_manager.load_with_overrides(
            client_id=cli_overrides.get("client_id"),
            client_secret=cli_overrides.get("client_secret"),
            api_url=cli_overrides.get("api_url"),
            org_name=org,
        )
        org_config = config_manager.get_org_config(config, org)

        with PortAPIClient(
            client_id=org_config.client_id,
            client_secret=org_config.client_secret,
            api_url=org_config.api_url
        ) as client:
            client.delete_entity(blueprint_id, entity_id)
            console.print(f"[green]✓[/green] Entity '{entity_id}' deleted successfully!")

    except Exception as e:
        console.print(f"[red]✗[/red] Failed to delete entity: {e}")
        raise typer.Exit(code=1)

