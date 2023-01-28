use std::vec;

use serde::{Deserialize, Serialize};
use strum_macros::Display;

#[derive(Serialize, Deserialize, Debug, Display)]
#[serde(rename_all = "lowercase")]
#[strum(serialize_all = "lowercase")]
pub enum Language {
    Go,
    Nix,
    Rust,
    TypeScript,
}

#[derive(Serialize, Deserialize, Debug, Display)]
#[serde(rename_all = "lowercase")]
#[strum(serialize_all = "lowercase")]
pub enum Template {
    Pulumi,
}

#[derive(Serialize, Deserialize, Debug)]
pub struct Configuration {
    pub name: String,
    pub language: Language,
    pub template: Template,

    #[serde(default = "default_registry")]
    pub registry: String,
}

#[derive(Debug)]
pub struct TemplateFile<'a> {
    pub data: &'a str,
    pub name: &'a str,
    pub path: Option<&'a str>,
}

fn default_registry() -> String {
    "677459762413.dkr.ecr.us-west-2.amazonaws.com".to_string()
}

pub fn get_files(config: &Configuration) -> Vec<TemplateFile> {
    match config.template {
        Template::Pulumi => {
            vec![
                TemplateFile {
                    data: include_str!("template/pulumi/.circleci/config.yml"),
                    name: "config.yml",
                    path: Some(".circleci"),
                },
                TemplateFile {
                    data: include_str!("template/pulumi/justfile"),
                    name: "justfile",
                    path: None,
                },
            ]
        }
    }
}
