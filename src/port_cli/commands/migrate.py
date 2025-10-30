"""Migration commands."""

from typing import Optional

import typer
from rich.console import Console
from rich.progress import Progress, SpinnerColumn, TextColumn

from port_cli.config import ConfigManager
from port_cli.modules.migrate_py import MigrateModule

console = Console()
app = typer.Typer(help="Migrate data between Port organizations")


@app.command()
def run(
    ctx: typer.Context,
    source_org: str = typer.Option(
        ...,
        "--source-org",
        "-s",
        help="Source organization name",
    ),
    target_org: str = typer.Option(
        ...,
        "--target-org",
        "-t",
        help="Target organization name",
    ),
    blueprints: Optional[str] = typer.Option(
        None,
        "--blueprints",
        "-b",
        help="Comma-separated list of blueprint IDs to migrate (migrates all if not specified)",
    ),
    dry_run: bool = typer.Option(
        False,
        "--dry-run",
        help="Validate migration without applying changes",
    ),
) -> None:
    """
    Migrate data between Port organizations.

    Migrates blueprints, entities, scorecards, actions, and teams from source to target organization.
    """
    try:
        # Get CLI overrides from context
        cli_overrides = ctx.obj or {}
        
        # Load configuration
        config_manager = ConfigManager(cli_overrides.get("config_file"))
        
        # For migrate, CLI overrides don't make sense (needs 2 orgs)
        # So we just load normally but warn if CLI creds are provided
        if cli_overrides.get("client_id") or cli_overrides.get("client_secret"):
            console.print(
                "[yellow]Warning:[/yellow] CLI credential flags are ignored for migrate command. "
                "Use config file or environment variables to specify source and target organizations."
            )
        
        config = config_manager.load()

        # Get source and target configurations
        source_config = config_manager.get_org_config(config, source_org)
        target_config = config_manager.get_org_config(config, target_org)

        # Parse blueprints list
        blueprint_list = None
        if blueprints:
            blueprint_list = [b.strip() for b in blueprints.split(",")]

        # Create migration module
        migrate_module = MigrateModule(source_config, target_config)

        # Start migration
        console.print(f"\n[bold]Migration:[/bold]")
        console.print(f"  Source: [cyan]{source_org}[/cyan]")
        console.print(f"  Target: [cyan]{target_org}[/cyan]")
        if blueprint_list:
            console.print(f"  Blueprints: [dim]{', '.join(blueprint_list)}[/dim]")
        if dry_run:
            console.print("[yellow]⚠[/yellow]  Dry run mode - no changes will be applied")

        with Progress(
            SpinnerColumn(),
            TextColumn("[progress.description]{task.description}"),
            console=console,
        ) as progress:
            progress.add_task("Migrating data...", total=None)

            result = migrate_module.execute(
                blueprints=blueprint_list,
                dry_run=dry_run,
            )

        if result.success:
            console.print(f"\n[green]✓[/green] Migration completed successfully!")
            console.print(f"[dim]{result.message}[/dim]")
            if result.data:
                for key, value in result.data.items():
                    if not key.startswith("errors"):
                        console.print(f"[dim]{key}: {value}[/dim]")
                        
                # Show errors if any
                if result.data.get("errors"):
                    console.print(f"\n[yellow]Warnings:[/yellow]")
                    for error in result.data["errors"][:5]:  # Show first 5
                        console.print(f"  [dim]{error}[/dim]")
                    if len(result.data["errors"]) > 5:
                        console.print(f"  [dim]... and {len(result.data['errors']) - 5} more[/dim]")
        else:
            console.print(f"\n[red]✗[/red] Migration failed: {result.error}")
            raise typer.Exit(code=1)

    except Exception as e:
        console.print(f"\n[red]✗[/red] Migration failed: {e}")
        raise typer.Exit(code=1)

