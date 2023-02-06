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
    Pulumi,
    Service,
}

#[derive(Serialize, Deserialize, Debug)]
pub struct Service {
    #[serde(default = "default_service_database")]
    pub database: bool,
}

#[derive(Serialize, Deserialize, Debug)]
pub struct Pulumi {
    #[serde(default = "default_pulumi_eks_cluster")]
    pub eks_cluster: String,

    #[serde(default = "default_pulumi_npm")]
    pub npm: bool,

    #[serde(default = "default_pulumi_resource_class")]
    pub resource_class: String,
}

#[derive(Serialize, Deserialize, Debug)]
pub struct Dockerfile {
    #[serde(default = "default_dockerfile_build_post_install")]
    pub build_post_install: Vec<String>,

    #[serde(default = "default_dockerfile_registry")]
    pub registry: String,

    #[serde(default = "default_dockerfile_service_post_install")]
    pub service_post_install: Vec<String>,
}

#[derive(Serialize, Deserialize, Debug)]
pub struct Configuration {
    pub name: String,
    pub language: Language,
    pub template: Template,

    #[serde(default = "default_dockerfile")]
    pub dockerfile: Dockerfile,

    #[serde(default = "default_environments")]
    pub environments: Vec<String>,

    #[serde(default = "default_product")]
    pub product: Product,

    #[serde(default = "default_pulumi")]
    pub pulumi: Pulumi,

    #[serde(default = "default_service")]
    pub service: Service,
}

#[derive(Debug)]
pub struct TemplateFile<'a> {
    pub data: &'a str,
    pub name: &'a str,
    pub path: Option<&'a str>,
}

fn default_environments() -> Vec<String> {
    vec![]
}

fn default_dockerfile() -> Dockerfile {
    Dockerfile {
        build_post_install: default_dockerfile_build_post_install(),
        registry: default_dockerfile_registry(),
        service_post_install: default_dockerfile_service_post_install(),
    }
}

fn default_dockerfile_build_post_install() -> Vec<String> {
    vec![]
}

fn default_dockerfile_service_post_install() -> Vec<String> {
    vec![]
}

fn default_dockerfile_registry() -> String {
    "677459762413.dkr.ecr.us-west-2.amazonaws.com".to_string()
}

fn default_product() -> Product {
    Product::AltF4Llc
}

fn default_pulumi() -> Pulumi {
    Pulumi {
        eks_cluster: default_pulumi_eks_cluster(),
        npm: default_pulumi_npm(),
        resource_class: default_pulumi_resource_class(),
    }
}

fn default_pulumi_eks_cluster() -> String {
    "".to_string()
}

fn default_pulumi_npm() -> bool {
    false
}

fn default_pulumi_resource_class() -> String {
    "small".to_string()
}

fn default_service() -> Service {
    Service {
        database: default_service_database(),
    }
}

fn default_service_database() -> bool {
    false
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
                    data: include_str!("template/pulumi/.dockerignore"),
                    name: ".dockerignore",
                    path: None,
                },
                TemplateFile {
                    data: include_str!("template/pulumi/justfile"),
                    name: "justfile",
                    path: None,
                },
                TemplateFile {
                    data: include_str!("template/pulumi/Pulumi.yaml"),
                    name: "Pulumi.yaml",
                    path: None,
                },
                TemplateFile {
                    data: include_str!("template/pulumi/.gitignore"),
                    name: ".gitignore",
                    path: None,
                },
                TemplateFile {
                    data: include_str!("template/pulumi/Dockerfile"),
                    name: "Dockerfile",
                    path: None,
                },
                TemplateFile {
                    data: include_str!("template/pulumi/tsconfig.json"),
                    name: "tsconfig.json",
                    path: None,
                },
            ]
        }
        Template::Service => {
            vec![
                TemplateFile {
                    data: include_str!("template/service/Dockerfile"),
                    name: "Dockerfile",
                    path: None,
                },
                TemplateFile {
                    data: include_str!("template/service/.npmrc"),
                    name: ".npmrc",
                    path: None,
                },
                TemplateFile {
                    data: include_str!("template/service/justfile"),
                    name: "justfile",
                    path: None,
                },
            ]
        }
    }
}
