use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Debug)]
pub struct DevShell {
    #[serde(default = "default_build_inputs")]
    pub build_inputs: Vec<String>,

    #[serde(default = "default_native_build_inputs")]
    pub native_build_inputs: Vec<String>,
}

#[derive(Serialize, Deserialize, Debug)]
pub struct Package {
    #[serde(default = "default_build_inputs")]
    pub build_inputs: Vec<String>,

    #[serde(default = "default_native_build_inputs")]
    pub native_build_inputs: Vec<String>,

    #[serde(default = "default_vendor_sha256")]
    pub vendor_sha256: String,
}

#[derive(Serialize, Deserialize, Debug)]
pub struct Configuration {
    #[serde(default = "default_binary_cache")]
    pub binary_cache: bool,

    #[serde(default = "default_devshell")]
    pub devshell: DevShell,

    #[serde(default = "default_package")]
    pub package: Package,
}

fn default_binary_cache() -> bool {
    false
}

fn default_build_inputs() -> Vec<String> {
    vec![]
}

fn default_native_build_inputs() -> Vec<String> {
    vec![]
}

fn default_vendor_sha256() -> String {
    "".to_string()
}

pub fn default_devshell() -> DevShell {
    DevShell {
        build_inputs: default_build_inputs(),
        native_build_inputs: default_native_build_inputs(),
    }
}

pub fn default_package() -> Package {
    Package {
        build_inputs: default_build_inputs(),
        native_build_inputs: default_native_build_inputs(),
        vendor_sha256: default_vendor_sha256(),
    }
}

pub fn default_config() -> Configuration {
    Configuration {
        binary_cache: default_binary_cache(),
        devshell: default_devshell(),
        package: default_package(),
    }
}
