use clap::Parser;
use cli::{Cli, Commands};

mod cli;
mod schema;

fn main() {
    let c = Cli::parse();

    match &c.command {
        Commands::Generate { config } => {
            if let Some(c) = config {
                println!("{}", c)
            }
        }
    }
}
