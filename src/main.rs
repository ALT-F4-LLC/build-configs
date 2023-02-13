use anyhow::Result;

mod cli;
mod config;
mod template;

fn main() -> Result<()> {
    cli::run()
}
