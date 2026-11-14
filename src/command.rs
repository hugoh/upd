//! A command line, split into program and arguments the way a shell would.

use std::fmt;

#[derive(Debug, Clone, PartialEq, Eq, serde::Deserialize)]
#[serde(try_from = "String")]
pub struct Command {
    program: String,
    args: Vec<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum CommandError {
    #[error("command is empty")]
    Empty,
    #[error("command name cannot be empty")]
    EmptyProgram,
    #[error("cannot parse command: {0}")]
    Syntax(String),
}

impl TryFrom<String> for Command {
    type Error = CommandError;

    fn try_from(line: String) -> Result<Self, CommandError> {
        Self::parse(&line)
    }
}

impl Command {
    /// # Errors
    ///
    /// Fails on unbalanced quotes, or when there is no program to run.
    pub fn parse(line: &str) -> Result<Self, CommandError> {
        let mut words =
            shell_words::split(line).map_err(|err| CommandError::Syntax(err.to_string()))?;
        if words.is_empty() {
            return Err(CommandError::Empty);
        }
        let program = words.remove(0);
        if program.is_empty() {
            return Err(CommandError::EmptyProgram);
        }
        Ok(Self {
            program,
            args: words,
        })
    }

    #[must_use]
    pub fn program(&self) -> &str {
        &self.program
    }

    #[must_use]
    pub fn args(&self) -> &[String] {
        &self.args
    }
}

impl fmt::Display for Command {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let words = std::iter::once(&self.program).chain(&self.args);
        f.write_str(&shell_words::join(words))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn splits_program_and_arguments() {
        let command = Command::parse("reboot --now 'a b'").unwrap();

        assert_eq!(command.program(), "reboot");
        assert_eq!(command.args(), ["--now", "a b"]);
    }

    #[test]
    fn displays_in_a_form_that_parses_back() {
        let command = Command::parse("echo 'two words' x").unwrap();

        assert_eq!(command.to_string(), "echo 'two words' x");
        assert_eq!(Command::parse(&command.to_string()).unwrap(), command);
    }

    #[test]
    fn rejects_nothing_to_run() {
        assert_eq!(Command::parse(""), Err(CommandError::Empty));
        assert_eq!(Command::parse("   "), Err(CommandError::Empty));
        assert_eq!(Command::parse("'' arg"), Err(CommandError::EmptyProgram));
    }

    #[test]
    fn rejects_unbalanced_quotes() {
        assert!(matches!(
            Command::parse("echo 'oops"),
            Err(CommandError::Syntax(_))
        ));
    }
}
