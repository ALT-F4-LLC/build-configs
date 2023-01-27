use std::fs;

use anyhow::Result;
use clap::Parser;
use cli::{Cli, Commands};
use schema::Config;
use serde_json::from_str;
use tera::{Context, Tera};

mod cli;
mod schema;

fn main() -> Result<()> {
    let c = Cli::parse();
    let t = Tera::new("template/**")?;

    match &c.command {
        Commands::Generate { config } => {
            let mut config_path = "build.json";

            if let Some(c) = config {
                config_path = c;
            }

            let contents = fs::read_to_string(config_path)?;

            let config: Config = from_str(&contents).unwrap();

            let context = Context::from_serialize(&config)?;

            let template_file = format!("{}/.circleci/config.yml", config.template);

            let template = t.render(&template_file, &context)?;

            println!("{}", template);
        }
    }

    Ok(())
}
