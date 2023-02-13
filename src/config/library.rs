use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Debug)]
pub struct Configuration {
    #[serde(default = "default_library_resource_class")]
    pub resource_class: String,
}

fn default_library_resource_class() -> String {
    "small".to_string()
}

pub fn default_library() -> Configuration {
    Configuration {
        resource_class: default_library_resource_class(),
    }
}
