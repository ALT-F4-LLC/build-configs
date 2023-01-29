use std::fs::{self, create_dir_all, write};

use anyhow::Result;
use clap::Parser;
use cli::{BuildConfigs, Commands};
use config::{get_files, Configuration};
use serde_json::from_str;
use tera::{Context, Tera};

mod cli;
mod config;

fn main() -> Result<()> {
    let cli = BuildConfigs::parse();

    match &cli.command {
        Commands::Generate { config_path } => {
            let config_contents = fs::read_to_string(config_path)?;

            let config: Configuration = from_str(&config_contents)?;

            let templates = get_files(&config);

            for file in templates {
                let mut path = format!("./{}", file.name);

                if let Some(p) = file.path {
                    create_dir_all(&p)?;
                    path = format!("{}/{}", p, file.name);
                };

                let context = Context::from_serialize(&config)?;

                let data = Tera::one_off(&file.data, &context, true)?;

                write(path, data)?;
            }

            Ok(())
        }
    }
}
