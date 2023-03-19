use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Debug)]
pub struct Configuration {
    #[serde(default = "default_dockerfile_build_post_install")]
    pub build_post_install: Vec<String>,

    #[serde(default = "default_dockerfile_registry")]
    pub registry: String,

    #[serde(default = "default_dockerfile_service_post_install")]
    pub service_post_install: Vec<String>,
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

pub fn default_config() -> Configuration {
    Configuration {
        build_post_install: default_dockerfile_build_post_install(),
        registry: default_dockerfile_registry(),
        service_post_install: default_dockerfile_service_post_install(),
    }
}
