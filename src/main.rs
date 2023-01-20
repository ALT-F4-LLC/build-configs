use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Debug)]
enum Language {
    Go,
    Nix,
    Rust,
    TypeScript,
}

#[derive(Serialize, Deserialize, Debug)]
enum Template {
    Docker,
    Pulumi,
}

#[derive(Serialize, Deserialize, Debug)]
struct Config {
    name: String,
    language: Language,
    template: Template,
}

fn main() {
    let cmd = clap::Command::new("build-configs")
        .bin_name("build-configs")
        .subcommand_required(true)
        .subcommand(clap::command!("generate").arg(
            clap::arg!(--"config" <PATH>).value_parser(clap::value_parser!(std::path::PathBuf)),
        ));

    let matches = cmd.get_matches();

    match matches.subcommand() {
        Some(("generate", matches)) => matches,
        _ => unreachable!("clap should ensure we don't get here"),
    };

    let manifest_path = matches.get_one::<std::path::PathBuf>("config");

    println!("{:?}", manifest_path);
}
