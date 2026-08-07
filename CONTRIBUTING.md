# Contributing to go-cache

Thank you for your interest in contributing to go-cache! This document provides guidelines and information for contributors.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Project Overview](#project-overview)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Contributing Guidelines](#contributing-guidelines)
- [Code Style](#code-style)
- [Testing](#testing)
- [Pull Request Process](#pull-request-process)
- [Reporting Issues](#reporting-issues)
- [License](#license)

## Code of Conduct

This project is committed to providing a welcoming and inclusive environment for all contributors. By participating, you are expected to uphold our community standards of respect and collaboration.

## Project Overview

go-cache is a goroutine-safe generic flexible implementation of in-memory cache written in Go. It provides:

- Generic cache implementation with type safety
- Configurable TTL (Time To Live) for cache items
- Atomic operations for thread safety
- Loader functions for automatic value retrieval
- Flexible expiration policies

## Getting Started

### Prerequisites

- Go 1.26 or later
- Git

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/go-cache.git
   cd go-cache
   ```
3. Add the upstream remote:
   ```bash
   git remote add upstream https://github.com/tunein/go-cache.git
   ```

## Development Setup

1. Install dependencies:
   ```bash
   go mod download
   ```

2. Run tests to ensure everything is working:
   ```bash
   go test ./...
   ```

3. Run tests with coverage:
   ```bash
   go test -cover ./...
   ```

## Contributing Guidelines

### Before You Start

- Check existing issues and pull requests to avoid duplication
- Discuss significant changes in an issue before implementing
- Ensure your changes align with the project's goals and architecture

### Types of Contributions

We welcome various types of contributions:

- **Bug fixes**: Fix issues and improve reliability
- **Feature enhancements**: Add new functionality
- **Documentation**: Improve README, code comments, and examples
- **Tests**: Add or improve test coverage
- **Performance improvements**: Optimize existing code
- **Code quality**: Refactor and improve code structure

## Code Style

### Go Code Standards

- Follow [Effective Go](https://golang.org/doc/effective_go.html) guidelines
- Use `gofmt` to format your code
- Follow the existing code style and patterns in the project
- Use meaningful variable and function names
- Add comments for exported functions and complex logic

### File Organization

- Keep related functionality together
- Use clear, descriptive file names
- Group imports logically (standard library, third-party, local)

### Error Handling

- Always check and handle errors appropriately
- Use meaningful error messages
- Consider wrapping errors with context when appropriate

## Testing

### Test Requirements

- All new code must include tests
- Maintain or improve test coverage
- Tests should be clear and readable
- Use descriptive test names that explain the scenario

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test -cover ./...

# Run tests for a specific package
go test ./cache

# Run a specific test
go test -run TestCacheSet
```

### Test Structure

Follow the existing test patterns in the project:

```go
func TestFunctionName(t *testing.T) {
    // Arrange
    // ... setup code ...
    
    // Act
    // ... execute function ...
    
    // Assert
    // ... verify results ...
}
```

## Pull Request Process

### Before Submitting

1. Ensure your code compiles without errors
2. Run all tests and ensure they pass
3. Update documentation if needed
4. Follow the commit message conventions

### Commit Message Format

Use clear, descriptive commit messages:

```
type(scope): brief description

Detailed description if needed

Fixes #123
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

### Pull Request Checklist

- [ ] Code follows project style guidelines
- [ ] Tests pass and coverage is maintained
- [ ] Documentation is updated if needed
- [ ] Commit messages are clear and descriptive
- [ ] Changes are focused and well-described
- [ ] Any new dependencies are necessary and well-maintained

### Review Process

1. All pull requests require review from maintainers
2. Address feedback and requested changes
3. Maintainers may request additional changes or clarification
4. Once approved, your PR will be merged

## Reporting Issues

### Bug Reports

When reporting bugs, please include:

- Clear description of the problem
- Steps to reproduce the issue
- Expected vs. actual behavior
- Go version and operating system
- Code example that demonstrates the issue
- Any relevant error messages or logs

### Feature Requests

For feature requests:

- Describe the desired functionality
- Explain the use case and benefits
- Provide examples of how it would be used
- Consider implementation complexity and maintenance

## License

By contributing to go-cache, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE), the same license as the project.

## Getting Help

If you need help or have questions:

- Check existing issues and documentation
- Open a new issue for bugs or feature requests
- Ask questions in the issue tracker

## Recognition

Contributors will be recognized in the project's documentation and release notes. We appreciate all contributions, big and small!

---

Thank you for contributing to go-cache! Your efforts help make this project better for everyone.
