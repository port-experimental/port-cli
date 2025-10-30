"""Export and import commands."""

from pathlib import Path
from typing import Optional

import typer
from rich.console import Console
from rich.progress import Progress, SpinnerColumn, TextColumn

from port_cli.config import ConfigManager
from port_cli.modules.registry import ModuleRegistry

console = Console()
app = typer.Typer(help="Export data from Port")
import_app = typer.Typer(help="Import data to Port")


@app.command()
def run(
    ctx: typer.Context,
    output: Path = typer.Option(
        ...,
        "--output",
        "-o",
        help="Output file path (e.g., backup.tar.gz or backup.json)",
    ),
    org: Optional[str] = typer.Option(
        None,
        "--org",
        help="Organization name (uses default if not specified)",
    ),
    blueprints: Optional[str] = typer.Option(
        None,
        "--blueprints",
        "-b",
        help="Comma-separated list of blueprint IDs to export (exports all if not specified)",
    ),
    format: str = typer.Option(
        "tar",
        "--format",
        "-f",
        help="Export format: tar (tar.gz) or json",
    ),
    skip_entities: bool = typer.Option(
        False,
        "--skip-entities",
        help="Skip exporting entities (only export schema and configuration)",
    ),
    include: Optional[str] = typer.Option(
        None,
        "--include",
        help="Comma-separated list of resources to export (e.g., 'blueprints,pages'). "
             "Available: blueprints, entities, scorecards, actions, teams, automations, pages, integrations. "
             "If not specified, exports all resources.",
    ),
) -> None:
    """
    Export data from Port organization.

    Exports blueprints, entities, scorecards, actions, and teams to a file.
    Use --skip-entities to only export configuration without entity data.
    Use --include to selectively export specific resource types.
    """
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

        # Parse blueprints list
        blueprint_list = None
        if blueprints:
            blueprint_list = [b.strip() for b in blueprints.split(",")]

        # Parse include list
        include_list = None
        if include:
            valid_resources = {"blueprints", "entities", "scorecards", "actions", "teams", 
                             "automations", "pages", "integrations"}
            include_list = [r.strip() for r in include.split(",")]
            invalid = set(include_list) - valid_resources
            if invalid:
                console.print(f"[red]✗[/red] Invalid resources: {', '.join(invalid)}")
                console.print(f"[dim]Valid resources: {', '.join(sorted(valid_resources))}[/dim]")
                raise typer.Exit(code=1)
            
            # Handle conflict between skip_entities and include
            if skip_entities and "entities" in include_list:
                console.print("[yellow]⚠[/yellow] --skip-entities conflicts with --include entities, ignoring --skip-entities")
                skip_entities = False

        # Get export module
        registry = ModuleRegistry()
        export_module = registry.get_module("export", org_config)
        
        if not export_module:
            console.print("[red]✗[/red] Export module not available")
            raise typer.Exit(code=1)

        # Export data
        console.print(f"\n[bold]Exporting data to:[/bold] [cyan]{output}[/cyan]")
        if blueprint_list:
            console.print(f"[dim]Blueprints filter: {', '.join(blueprint_list)}[/dim]")
        if include_list:
            console.print(f"[cyan]ℹ[/cyan] Including only: {', '.join(include_list)}")
        elif skip_entities:
            console.print(f"[yellow]⚠[/yellow] Skipping entities (schema only)")

        with Progress(
            SpinnerColumn(),
            TextColumn("[progress.description]{task.description}"),
            console=console,
        ) as progress:
            progress.add_task("Exporting data...", total=None)

            result = export_module.execute(
                output_path=str(output),
                blueprints=blueprint_list,
                format=format,
                skip_entities=skip_entities,
                include_resources=include_list,
            )

        if result.success:
            console.print(f"\n[green]✓[/green] Export completed successfully!")
            console.print(f"[dim]{result.message}[/dim]")
            if result.data:
                console.print(f"[dim]Blueprints: {result.data.get('blueprints_count', 0)}[/dim]")
                console.print(f"[dim]Entities: {result.data.get('entities_count', 0)}[/dim]")
                console.print(f"[dim]Automations: {result.data.get('automations_count', 0)}[/dim]")
                console.print(f"[dim]Pages: {result.data.get('pages_count', 0)}[/dim]")
                console.print(f"[dim]Integrations: {result.data.get('integrations_count', 0)}[/dim]")
        else:
            console.print(f"\n[red]✗[/red] Export failed: {result.error}")
            raise typer.Exit(code=1)

    except Exception as e:
        console.print(f"\n[red]✗[/red] Export failed: {e}")
        raise typer.Exit(code=1)


@import_app.command("run")
def import_run(
    ctx: typer.Context,
    input: Path = typer.Option(
        ...,
        "--input",
        "-i",
        help="Input file path (e.g., backup.tar.gz or backup.json)",
    ),
    org: Optional[str] = typer.Option(
        None,
        "--org",
        help="Organization name (uses default if not specified)",
    ),
    client_id: Optional[str] = typer.Option(
        None,
        "--client-id",
        help="Port API Client ID (overrides environment variable)",
    ),
    client_secret: Optional[str] = typer.Option(
        None,
        "--client-secret",
        help="Port API Client Secret (overrides environment variable)",
    ),
    dry_run: bool = typer.Option(
        False,
        "--dry-run",
        help="Validate import without applying changes",
    ),
    skip_entities: bool = typer.Option(
        False,
        "--skip-entities",
        help="Skip importing entities (only import schema and configuration)",
    ),
    include: Optional[str] = typer.Option(
        None,
        "--include",
        help="Comma-separated list of resources to import (e.g., 'blueprints,pages'). "
             "Available: blueprints, entities, scorecards, actions, teams, automations, pages, integrations. "
             "If not specified, imports all resources.",
    ),
) -> None:
    """
    Import data to Port organization.

    Imports blueprints, entities, scorecards, actions, teams, automations, pages, and integrations from a file.
    Use --skip-entities to only import configuration without entity data.
    Use --include to selectively import specific resource types.
    """
    try:
        # Check if input file exists
        if not input.exists():
            console.print(f"[red]✗[/red] Input file not found: {input}")
            raise typer.Exit(code=1)

        # Get CLI overrides from context
        cli_overrides = ctx.obj or {}
        
        # Load configuration with CLI overrides (command flags take precedence)
        config_manager = ConfigManager(cli_overrides.get("config_file"))
        config = config_manager.load_with_overrides(
            client_id=client_id or cli_overrides.get("client_id"),
            client_secret=client_secret or cli_overrides.get("client_secret"),
            api_url=cli_overrides.get("api_url"),
            org_name=org,
        )
        org_config = config_manager.get_org_config(config, org)

        # Parse include list
        include_list = None
        if include:
            valid_resources = {"blueprints", "entities", "scorecards", "actions", "teams",
                             "automations", "pages", "integrations"}
            include_list = [r.strip() for r in include.split(",")]
            invalid = set(include_list) - valid_resources
            if invalid:
                console.print(f"[red]✗[/red] Invalid resources: {', '.join(invalid)}")
                console.print(f"[dim]Valid resources: {', '.join(sorted(valid_resources))}[/dim]")
                raise typer.Exit(code=1)
            
            # Handle conflict
            if skip_entities and "entities" in include_list:
                console.print("[yellow]⚠[/yellow] --skip-entities conflicts with --include entities, ignoring --skip-entities")
                skip_entities = False

        # Get import module
        registry = ModuleRegistry()
        import_module = registry.get_module("import", org_config)
        
        if not import_module:
            console.print("[red]✗[/red] Import module not available")
            raise typer.Exit(code=1)

        # Import data
        console.print(f"\n[bold]Importing data from:[/bold] [cyan]{input}[/cyan]")
        if dry_run:
            console.print("[yellow]⚠[/yellow] Dry run mode - no changes will be applied")
        if include_list:
            console.print(f"[cyan]ℹ[/cyan] Including only: {', '.join(include_list)}")
        elif skip_entities:
            console.print(f"[yellow]⚠[/yellow] Skipping entities (schema only)")

        with Progress(
            SpinnerColumn(),
            TextColumn("[progress.description]{task.description}"),
            console=console,
        ) as progress:
            progress.add_task("Importing data...", total=None)

            result = import_module.execute(
                input_path=str(input),
                dry_run=dry_run,
                skip_entities=skip_entities,
                include_resources=include_list,
            )

        if result.success:
            console.print(f"\n[green]✓[/green] Import completed successfully!")
            console.print(f"[dim]{result.message}[/dim]")
            if result.data:
                console.print(f"[dim]Blueprints: {result.data.get('blueprints_created', 0)} created, {result.data.get('blueprints_updated', 0)} updated[/dim]")
                console.print(f"[dim]Entities: {result.data.get('entities_created', 0)} created, {result.data.get('entities_updated', 0)} updated[/dim]")
                console.print(f"[dim]Scorecards: {result.data.get('scorecards_created', 0)} created, {result.data.get('scorecards_updated', 0)} updated[/dim]")
                console.print(f"[dim]Actions: {result.data.get('actions_created', 0)} created, {result.data.get('actions_updated', 0)} updated[/dim]")
                console.print(f"[dim]Teams: {result.data.get('teams_created', 0)} created, {result.data.get('teams_updated', 0)} updated[/dim]")
                console.print(f"[dim]Automations: {result.data.get('automations_created', 0)} created, {result.data.get('automations_updated', 0)} updated[/dim]")
                console.print(f"[dim]Pages: {result.data.get('pages_created', 0)} created, {result.data.get('pages_updated', 0)} updated[/dim]")
                console.print(f"[dim]Integrations: {result.data.get('integrations_updated', 0)} updated[/dim]")
                if result.data.get("errors"):
                    error_count = len(result.data['errors'])
                    console.print(f"\n[yellow]⚠[/yellow] [yellow]{error_count} errors occurred during import:[/yellow]")
                    # Show first 10 errors
                    for error in result.data['errors'][:10]:
                        console.print(f"[dim]  • {error}[/dim]")
                    if error_count > 10:
                        console.print(f"[dim]  ... and {error_count - 10} more errors[/dim]")
        else:
            console.print(f"\n[red]✗[/red] Import failed: {result.error}")
            raise typer.Exit(code=1)

    except Exception as e:
        console.print(f"\n[red]✗[/red] Import failed: {e}")
        raise typer.Exit(code=1)

