use std::{
    fs::{create_dir_all, write},
    path::Path,
};

use anyhow::Result;
use tera::{Context, Tera};

use crate::config::{Configuration, Language, Template};

#[derive(Debug)]
pub struct TemplateFile<'a> {
    pub name: &'a str,
}

pub fn get_templates(config: &Configuration) -> Vec<(&str, &str)> {
    match config.template {
        Template::Library => match config.language {
            Language::Go => {
                vec![]
            }
            Language::Rust => {
                vec![]
            }
            Language::TypeScript => {
                vec![
                    (
                        ".circleci/config.yml",
                        include_str!("./template/library/typescript/.circleci/config.yml.j2"),
                    ),
                    (
                        "Dockerfile",
                        include_str!("./template/library/typescript/Dockerfile.j2"),
                    ),
                    (
                        ".eslintignore",
                        include_str!("./template/library/typescript/Dockerfile.j2"),
                    ),
                    (
                        ".eslintrc.js",
                        include_str!("./template/library/typescript/Dockerfile.j2"),
                    ),
                    (
                        ".gitignore",
                        include_str!("./template/library/typescript/.gitignore.j2"),
                    ),
                    (
                        "jest.config.js",
                        include_str!("./template/library/typescript/jest.config.js.j2"),
                    ),
                    (
                        "justfile",
                        include_str!("./template/library/typescript/justfile.j2"),
                    ),
                    (
                        ".npmignore",
                        include_str!("./template/library/typescript/.npmignore.j2"),
                    ),
                    (
                        ".prettierignore",
                        include_str!("./template/library/typescript/.prettierignore.j2"),
                    ),
                    (
                        ".prettierrc.js",
                        include_str!("./template/library/typescript/.prettierrc.js.j2"),
                    ),
                    (
                        "tsconfig.json",
                        include_str!("./template/library/typescript/tsconfig.json.j2"),
                    ),
                ]
            }
        },
        Template::Pulumi => match config.language {
            Language::Go => {
                vec![]
            }
            Language::Rust => {
                vec![]
            }
            Language::TypeScript => {
                vec![
                    (
                        ".circleci/config.yml",
                        include_str!("./template/pulumi/typescript/.circleci/config.yml.j2"),
                    ),
                    (
                        ".dockerignore",
                        include_str!("./template/pulumi/typescript/.dockerignore.j2"),
                    ),
                    (
                        ".gitignore",
                        include_str!("./template/pulumi/typescript/.gitignore.j2"),
                    ),
                    (
                        "Dockerfile",
                        include_str!("./template/pulumi/typescript/Dockerfile.j2"),
                    ),
                    (
                        "Pulumi.yaml",
                        include_str!("./template/pulumi/typescript/Pulumi.yaml.j2"),
                    ),
                    (
                        "justfile",
                        include_str!("./template/pulumi/typescript/justfile.j2"),
                    ),
                    (
                        "tsconfig.json",
                        include_str!("./template/pulumi/typescript/tsconfig.json.j2"),
                    ),
                ]
            }
        },
        Template::Service => match config.language {
            Language::Go => {
                vec![
                    (
                        ".github/workflows/build-and-release.yml",
                        include_str!(
                            "./template/service/go/.github/workflows/build-and-release.yml.j2"
                        ),
                    ),
                    (
                        "flake.nix",
                        include_str!("./template/service/go/flake.nix.j2"),
                    ),
                    (
                        ".gitignore",
                        include_str!("./template/service/go/.gitignore.j2"),
                    ),
                ]
            }
            Language::Rust => {
                vec![]
            }
            Language::TypeScript => {
                vec![
                    (
                        ".circleci/config.yml",
                        include_str!("./template/service/typescript/.circleci/config.yml.j2"),
                    ),
                    (
                        "Dockerfile",
                        include_str!("./template/service/typescript/Dockerfile.j2"),
                    ),
                    (
                        ".dockerignore",
                        include_str!("./template/service/typescript/.dockerignore.j2"),
                    ),
                    (
                        ".eslintignore",
                        include_str!("./template/service/typescript/.eslintignore.j2"),
                    ),
                    (
                        ".eslintrc.js",
                        include_str!("./template/service/typescript/.eslintrc.js.j2"),
                    ),
                    (
                        "justfile",
                        include_str!("./template/service/typescript/justfile.j2"),
                    ),
                    (
                        ".npmrc",
                        include_str!("./template/service/typescript/.npmrc.j2"),
                    ),
                    (
                        "docker-entrypoint.sh",
                        include_str!("./template/service/typescript/docker-entrypoint.sh.j2"),
                    ),
                    (
                        "jest.config.js",
                        include_str!("./template/service/typescript/jest.config.js.j2"),
                    ),
                    (
                        ".prettierignore",
                        include_str!("./template/service/typescript/.prettierignore.j2"),
                    ),
                    (
                        ".prettierrc.js",
                        include_str!("./template/service/typescript/.prettierrc.js.j2"),
                    ),
                    (
                        "tsconfig.json",
                        include_str!("./template/service/typescript/tsconfig.json.j2"),
                    ),
                ]
            }
        },
    }
}

pub fn render_templates(config: &Configuration) -> Result<()> {
    let template_files = get_templates(&config);

    let mut tera = Tera::default();

    tera.add_raw_templates(template_files.clone())?;

    let test_unit = "unit".to_string();

    let test_unit_included = config.service.tests.contains(&test_unit);

    for (key, _) in template_files {
        if key == ".npmrc" && !config.dependencies.private {
            continue;
        }

        if key == "jest.config.js" && !test_unit_included {
            continue;
        }

        if key == "tsconfig.json" && config.language == Language::Go {
            continue;
        }

        let context = Context::from_serialize(&config)?;

        let data = tera.render(key, &context)?;

        let path = Path::new(key);

        let parent = path.parent().unwrap().to_str().unwrap();

        if parent != "" {
            create_dir_all(parent)?;
        }

        write(path, data)?;
    }

    Ok(())
}
