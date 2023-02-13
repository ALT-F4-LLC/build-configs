use anyhow::Result;
use serde_json::from_str;
use std::fs::read_to_string;

use crate::{config::Configuration, template};

pub fn run(config_path: &str) -> Result<()> {
    let config_contents = read_to_string(config_path)?;

    let config: Configuration = from_str(&config_contents)?;

    template::render_templates(&config)?;

    Ok(())
}
