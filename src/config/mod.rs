use serde::{Deserialize, Serialize};
use std::vec;
use strum_macros::Display;

mod dockerfile;
mod library;
mod pulumi;
mod service;

#[derive(Serialize, Deserialize, Debug)]
pub struct Dependencies {
    #[serde(default = "default_dependencies_private")]
    pub private: bool,
}

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
pub enum Product {
    AltF4Llc,
    AyaLivingInc,
    Quirk,
    RecordingPipeline,
}

#[derive(Serialize, Deserialize, Debug, Display)]
#[serde(rename_all = "lowercase")]
#[strum(serialize_all = "lowercase")]
pub enum Template {
    Library,
    Pulumi,
    Service,
}

#[derive(Serialize, Deserialize, Debug)]
pub struct Configuration {
    pub name: String,
    pub language: Language,
    pub template: Template,

    #[serde(default = "default_dependencies")]
    pub dependencies: Dependencies,

    #[serde(default = "dockerfile::default_config")]
    pub dockerfile: dockerfile::Configuration,

    #[serde(default = "default_environments")]
    pub environments: Vec<String>,

    #[serde(default = "library::default_library")]
    pub library: library::Configuration,

    #[serde(default = "default_product")]
    pub product: Product,

    #[serde(default = "pulumi::default_pulumi")]
    pub pulumi: pulumi::Configuration,

    #[serde(default = "service::default_config")]
    pub service: service::Configuration,
}

fn default_dependencies_private() -> bool {
    false
}

fn default_dependencies() -> Dependencies {
    Dependencies {
        private: default_dependencies_private(),
    }
}

fn default_environments() -> Vec<String> {
    vec![]
}

fn default_product() -> Product {
    Product::AltF4Llc
}
