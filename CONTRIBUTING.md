# Contributing to Lookout

Thank you for your interest in contributing to Lookout! This document covers the contribution process and policies.

For development setup, coding standards, and testing, see [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).

## How to Contribute

1. **Open an issue** first to discuss the change you'd like to make
2. **Fork the repository** and create a feature branch from `main`
3. **Make your changes** following the coding standards in [DEVELOPMENT.md](docs/DEVELOPMENT.md)
4. **Write tests** for new functionality
5. **Run the test suite** and linter before submitting:
   ```bash
   make test
   golangci-lint run
   ```
6. **Submit a pull request** using the PR template

## Pull Request Guidelines

- Keep PRs focused on a single change
- Write a clear description of what the PR does and why
- Reference related issues (e.g., "Fixes #42")
- Ensure CI passes (tests, lint, security scan)
- Be responsive to review feedback

### What makes a good PR

- **Small and focused** - one logical change per PR
- **Well-tested** - new code has tests, existing tests still pass
- **Clear intent** - the reviewer can understand *why* the change was made
- **Clean history** - squash fixup commits before requesting review

### What will get a PR closed

- Drive-by PRs with no prior discussion on an issue
- PRs that don't pass CI
- Bulk changes with no clear rationale
- PRs where the author can't explain the changes when asked

## AI-Assisted Contributions Policy

We welcome contributions that use AI tools (GitHub Copilot, Claude, ChatGPT, etc.) as part of the development process. However, we require transparency and accountability.

### Disclosure Requirement

If AI tools were used in a meaningful way to generate, debug, or design your contribution, you must disclose this in your pull request. The PR template includes a required section for this.

**What requires disclosure:**
- Code generated or substantially written by AI
- Architecture or design decisions guided by AI
- Debugging assistance that led to the fix
- Documentation drafted by AI

**What does not require disclosure:**
- IDE autocomplete or simple code completion
- Syntax suggestions or variable name recommendations
- Using AI to understand existing code (reading, not writing)

### Quality Standards

AI-assisted contributions are held to the same quality standards as any other contribution:

- **You must understand the code you submit.** If you can't explain what it does and why during review, the PR will be closed.
- **You must have tested the code.** "The AI generated it" is not a substitute for verification.
- **You are responsible for correctness.** The contributor, not the AI tool, is accountable for bugs, security issues, and maintenance.

### What we will reject

- **AI slop**: bulk PRs that are clearly unreviewed AI output (formatting issues, hallucinated APIs, generic boilerplate that doesn't fit the project)
- **Unexplained changes**: if you can't articulate why a change is needed or how it works, it won't be merged regardless of how it was produced
- **Quantity over quality**: multiple low-effort AI-generated PRs are worse than one thoughtful contribution

## Reporting Issues

- Use [GitHub Issues](https://github.com/timoniersystems/lookout/issues) for bugs and feature requests
- For security vulnerabilities, see [SECURITY.md](SECURITY.md)
- Include reproduction steps for bugs
- Check existing issues before opening a new one

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
