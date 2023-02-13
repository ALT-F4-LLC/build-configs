use anyhow::Result;
use clap::{Parser, Subcommand};

mod generate;

#[derive(Parser)]
#[command(author, version, about, long_about = None)]
#[command(propagate_version = true)]
pub struct BuildConfigs {
    #[command(subcommand)]
    pub command: Commands,
}

#[derive(Subcommand)]
pub enum Commands {
    Generate {
        #[arg(short, long)]
        #[arg(default_value_t = String::from("build.json"))]
        config_path: String,
    },
}

pub fn run() -> Result<()> {
    let cli = BuildConfigs::parse();

    match &cli.command {
        Commands::Generate { config_path } => generate::run(config_path),
    }
}
