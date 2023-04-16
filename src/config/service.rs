use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Debug)]
pub struct Configuration {
    #[serde(default = "default_database")]
    pub database: bool,

    #[serde(default = "default_tests")]
    pub tests: Vec<String>,
}

fn default_database() -> bool {
    false
}

fn default_tests() -> Vec<String> {
    vec![]
}

pub fn default_config() -> Configuration {
    Configuration {
        database: default_database(),
        tests: default_tests(),
    }
}
