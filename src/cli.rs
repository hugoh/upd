use std::path::PathBuf;

use clap::Parser;

use crate::version::VERSION;

/// Tool to monitor if the network connection is up.
#[derive(Debug, Parser)]
#[command(name = "upd", version = VERSION)]
pub struct Cli {
    /// Use the specified TOML configuration file [default: the first that exists of
    /// ./.upd.toml, $`XDG_CONFIG_HOME/upd/upd.toml`, $`XDG_CONFIG_DIRS/upd/upd.toml` and
    /// /etc/upd/upd.toml].
    #[arg(short, long, value_name = "FILE", env = "UPD_CONFIG")]
    pub config: Option<PathBuf>,

    /// Display debugging output in the console.
    #[arg(short, long)]
    pub debug: bool,
}

#[cfg(test)]
mod tests {
    use super::*;
    use clap::CommandFactory;

    #[test]
    fn the_command_line_definition_is_consistent() {
        Cli::command().debug_assert();
    }

    #[test]
    fn the_config_file_is_optional() {
        let cli = Cli::try_parse_from(["upd"]).unwrap();

        assert_eq!(cli.config, None);
        assert!(!cli.debug);
    }

    #[test]
    fn accepts_short_and_long_options() {
        let short = Cli::try_parse_from(["upd", "-c", "a.toml", "-d"]).unwrap();
        let long = Cli::try_parse_from(["upd", "--config", "a.toml", "--debug"]).unwrap();

        assert_eq!(short.config, Some(PathBuf::from("a.toml")));
        assert!(short.debug);
        assert_eq!(long.config, short.config);
    }

    #[test]
    fn both_version_spellings_print_the_version() {
        for flag in ["-V", "--version"] {
            let err = Cli::try_parse_from(["upd", flag]).unwrap_err();

            assert_eq!(err.kind(), clap::error::ErrorKind::DisplayVersion);
            assert!(err.to_string().contains(VERSION));
        }
    }

    #[test]
    fn unknown_options_are_errors() {
        assert!(Cli::try_parse_from(["upd", "--nope"]).is_err());
    }
}
