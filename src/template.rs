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
        Template::Library => {
            vec![
                (
                    ".circleci/config.yml",
                    include_str!("./template/library/.circleci/config.yml.j2"),
                ),
                (
                    "Dockerfile",
                    include_str!("./template/library/Dockerfile.j2"),
                ),
                (
                    ".eslintignore",
                    include_str!("./template/library/Dockerfile.j2"),
                ),
                (
                    ".eslintrc.js",
                    include_str!("./template/library/Dockerfile.j2"),
                ),
                (
                    ".gitignore",
                    include_str!("./template/library/.gitignore.j2"),
                ),
                (
                    "jest.config.js",
                    include_str!("./template/library/jest.config.js.j2"),
                ),
                ("justfile", include_str!("./template/library/justfile.j2")),
                (
                    ".npmignore",
                    include_str!("./template/library/.npmignore.j2"),
                ),
                (
                    ".prettierignore",
                    include_str!("./template/library/.prettierignore.j2"),
                ),
                (
                    ".prettierrc.js",
                    include_str!("./template/library/.prettierrc.js.j2"),
                ),
                (
                    "tsconfig.json",
                    include_str!("./template/library/tsconfig.json.j2"),
                ),
            ]
        }
        Template::Pulumi => {
            vec![
                (
                    ".circleci/config.yml",
                    include_str!("./template/pulumi/.circleci/config.yml.j2"),
                ),
                (
                    ".dockerignore",
                    include_str!("./template/pulumi/.dockerignore.j2"),
                ),
                (
                    ".gitignore",
                    include_str!("./template/pulumi/.gitignore.j2"),
                ),
                (
                    "Dockerfile",
                    include_str!("./template/pulumi/Dockerfile.j2"),
                ),
                (
                    "Pulumi.yaml",
                    include_str!("./template/pulumi/Pulumi.yaml.j2"),
                ),
                ("justfile", include_str!("./template/pulumi/justfile.j2")),
                (
                    "tsconfig.json",
                    include_str!("./template/pulumi/tsconfig.json.j2"),
                ),
            ]
        }
        Template::Service => {
            vec![
                (
                    ".circleci/config.yml",
                    include_str!("./template/service/.circleci/config.yml.j2"),
                ),
                (
                    "Dockerfile",
                    include_str!("./template/service/Dockerfile.j2"),
                ),
                (
                    ".dockerignore",
                    include_str!("./template/service/.dockerignore.j2"),
                ),
                (
                    ".eslintignore",
                    include_str!("./template/service/.eslintignore.j2"),
                ),
                (
                    ".eslintrc.js",
                    include_str!("./template/service/.eslintrc.js.j2"),
                ),
                ("justfile", include_str!("./template/service/justfile.j2")),
                (".npmrc", include_str!("./template/service/.npmrc.j2")),
                (
                    "docker-entrypoint.sh",
                    include_str!("./template/service/docker-entrypoint.sh.j2"),
                ),
                (
                    "jest.config.js",
                    include_str!("./template/service/jest.config.js.j2"),
                ),
                (
                    ".prettierignore",
                    include_str!("./template/service/.prettierignore.j2"),
                ),
                (
                    ".prettierrc.js",
                    include_str!("./template/service/.prettierrc.js.j2"),
                ),
                (
                    "tsconfig.json",
                    include_str!("./template/service/tsconfig.json.j2"),
                ),
            ]
        }
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
