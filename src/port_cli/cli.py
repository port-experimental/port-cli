"""Main CLI application using Typer."""

import sys
from typing import Optional

import typer
from rich.console import Console
from rich.traceback import install

from port_cli import __version__
from port_cli.commands import api, export_import, migrate
from port_cli.config import ConfigManager

# Install rich traceback handler
install(show_locals=True)

# Create console for rich output
console = Console()

# Create main Typer app
app = typer.Typer(
    name="port",
    help="Port CLI - Modular command-line interface for Port",
    add_completion=True,
    rich_markup_mode="rich",
)

# Add subcommands
app.add_typer(export_import.app, name="export", help="Export data from Port")
app.add_typer(export_import.import_app, name="import", help="Import data to Port")
app.add_typer(migrate.app, name="migrate", help="Migrate data between Port organizations")
app.add_typer(api.app, name="api", help="Direct Port API operations")


@app.command()
def version() -> None:
    """Show the CLI version."""
    console.print(f"[bold blue]Port CLI[/bold blue] version [green]{__version__}[/green]")


@app.command()
def config(
    show: bool = typer.Option(False, "--show", help="Show current configuration"),
    init: bool = typer.Option(False, "--init", help="Initialize configuration file"),
) -> None:
    """Manage Port CLI configuration."""
    config_manager = ConfigManager()

    if init:
        try:
            config_manager.create_default_config()
            console.print(
                f"[green]✓[/green] Configuration file created at "
                f"[cyan]{config_manager.config_path}[/cyan]"
            )
            console.print("\nPlease edit the file and add your Port credentials.")
        except Exception as e:
            console.print(f"[red]✗[/red] Failed to create configuration: {e}", stderr=True)
            raise typer.Exit(code=1)

    elif show:
        try:
            cfg = config_manager.load()
            console.print("\n[bold]Current Configuration:[/bold]\n")
            console.print(f"Config file: [cyan]{config_manager.config_path}[/cyan]")
            console.print(f"Default org: [yellow]{cfg.default_org or 'None'}[/yellow]")
            console.print(
                f"Backend URL: [yellow]{cfg.backend.url}[/yellow]"
            )
            console.print(f"Organizations: [yellow]{len(cfg.organizations)}[/yellow]")
            for org_name in cfg.organizations.keys():
                console.print(f"  - {org_name}")
        except Exception as e:
            console.print(f"[red]✗[/red] Failed to load configuration: {e}", stderr=True)
            raise typer.Exit(code=1)
    else:
        console.print("Use [cyan]--show[/cyan] to display configuration")
        console.print("Use [cyan]--init[/cyan] to create a new configuration file")


@app.callback()
def main(
    ctx: typer.Context,
    config_file: Optional[str] = typer.Option(
        None,
        "--config",
        "-c",
        help="Path to configuration file",
        envvar="PORT_CONFIG_FILE",
    ),
    client_id: Optional[str] = typer.Option(
        None,
        "--client-id",
        help="Port API client ID (overrides config/env)",
        envvar="PORT_CLIENT_ID",
    ),
    client_secret: Optional[str] = typer.Option(
        None,
        "--client-secret",
        help="Port API client secret (overrides config/env)",
        envvar="PORT_CLIENT_SECRET",
    ),
    api_url: Optional[str] = typer.Option(
        None,
        "--api-url",
        help="Port API URL (overrides config/env)",
        envvar="PORT_API_URL",
    ),
    debug: bool = typer.Option(
        False,
        "--debug",
        "-d",
        help="Enable debug mode",
        envvar="PORT_DEBUG",
    ),
) -> None:
    """
    Port CLI - Modular command-line interface for Port.

    Manage your Port organization with import/export, migration, and API operations.
    
    Credentials can be provided via:
      1. CLI flags (--client-id, --client-secret) - highest priority
      2. Environment variables (PORT_CLIENT_ID, PORT_CLIENT_SECRET)
      3. Configuration file (~/.port/config.yaml)
    """
    # Store global options in context
    ctx.obj = {
        "config_file": config_file,
        "client_id": client_id,
        "client_secret": client_secret,
        "api_url": api_url,
        "debug": debug,
    }

    if debug:
        console.print("[dim]Debug mode enabled[/dim]")
        if client_id:
            console.print(f"[dim]Using CLI client_id: {client_id[:8]}...[/dim]")


def run() -> None:
    """Entry point for the CLI."""
    try:
        app()
    except KeyboardInterrupt:
        console.print("\n[yellow]Operation cancelled by user[/yellow]")
        sys.exit(130)
    except Exception as e:
        console.print(f"\n[red]Error:[/red] {e}")
        sys.exit(1)


if __name__ == "__main__":
    run()

