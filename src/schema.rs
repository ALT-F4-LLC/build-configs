use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Debug)]
pub enum Language {
    Go,
    Nix,
    Rust,
    TypeScript,
}

#[derive(Serialize, Deserialize, Debug)]
pub enum Template {
    Docker,
    Pulumi,
}

#[derive(Serialize, Deserialize, Debug)]
pub struct Config {
    name: String,
    language: Language,
    template: Template,
}
