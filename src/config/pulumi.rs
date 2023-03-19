use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Debug)]
pub struct Configuration {
    #[serde(default = "default_eks_cluster")]
    pub eks_cluster: String,

    #[serde(default = "default_npm")]
    pub npm: bool,

    #[serde(default = "default_resource_class")]
    pub resource_class: String,
}

fn default_eks_cluster() -> String {
    "".to_string()
}

fn default_npm() -> bool {
    false
}

fn default_resource_class() -> String {
    "small".to_string()
}

pub fn default_pulumi() -> Configuration {
    Configuration {
        eks_cluster: default_eks_cluster(),
        npm: default_npm(),
        resource_class: default_resource_class(),
    }
}
