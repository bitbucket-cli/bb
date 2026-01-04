# Bitbucket CLI Roadmap

This document outlines planned features and improvements for the Bitbucket CLI (`bb`).

## Current Status

The CLI currently supports:
- **Authentication**: OAuth login/logout with secure token storage
- **Repositories**: List, view, clone, create, update, fork, delete
- **Pull Requests**: List, view, create, merge, approve, comment, diff
- **Branches**: List, view, create, delete, restrictions
- **Pipelines**: List, view, trigger, stop, steps, logs
- **Issues**: List, view, create, update, delete, comments
- **Tags**: List, view, create, delete
- **Webhooks**: List, view, create, delete
- **Deployments**: List environments, create, delete
- **Workspaces**: List, view, members, projects
- **Configuration**: Manage CLI settings
- **Shell Completions**: Bash, Zsh, Fish, PowerShell

## Planned Features

### High Priority

#### Git Integration
- [ ] Auto-detect workspace/repo from current git directory
- [ ] `bb pr checkout <pr-number>` - checkout PR branch locally
- [ ] `bb pr create` from current branch without specifying source

#### Enhanced Interactive Mode
- [ ] Add `-i` flag to more commands (branch, issue, etc.)
- [ ] Interactive PR creation wizard
- [ ] Interactive branch selection for operations

#### Pipeline Improvements
- [ ] Stream pipeline logs in real-time
- [ ] Pipeline variable management
- [ ] Schedule management
- [ ] Caches and artifacts management

### Medium Priority

#### Code Review Features
- [ ] Inline comments support for PR reviews
- [ ] PR review threads navigation
- [ ] Request changes / approve with comments
- [ ] PR templates support

#### Repository Management
- [ ] Repository settings management
- [ ] Access keys (SSH keys) management
- [ ] Repository variables for pipelines
- [ ] Branch merge strategies configuration

#### Snippets Support
- [ ] List snippets
- [ ] Create snippets from files or stdin
- [ ] View snippet content
- [ ] Update and delete snippets

#### Downloads
- [ ] List repository downloads
- [ ] Upload files to downloads
- [ ] Delete downloads

### Low Priority

#### Reports and Analytics
- [ ] Pipeline analytics and statistics
- [ ] Commit activity reports
- [ ] PR metrics (time to merge, etc.)

#### Bulk Operations
- [ ] Bulk PR operations (approve multiple, merge multiple)
- [ ] Bulk issue updates
- [ ] Export operations (PRs, issues to CSV/JSON)

#### Notifications
- [ ] List notifications
- [ ] Mark notifications as read
- [ ] Notification preferences

## Technical Improvements

### Testing
- [ ] Increase test coverage to 90%+
- [ ] Add integration tests with mock API server
- [ ] Add end-to-end tests

### Documentation
- [ ] Man pages generation
- [ ] Comprehensive README with examples
- [ ] Contributing guide
- [ ] API documentation

### Build and Distribution
- [ ] GitHub Actions CI/CD pipeline
- [ ] Cross-platform binaries (Linux, macOS, Windows)
- [ ] Homebrew formula
- [ ] APT/RPM packages
- [ ] Scoop manifest for Windows

### Performance
- [ ] Response caching for frequently accessed data
- [ ] Concurrent API requests where applicable
- [ ] Lazy loading for large result sets

### Code Quality
- [ ] Golangci-lint integration
- [ ] Pre-commit hooks
- [ ] Semantic versioning
- [ ] Changelog automation

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests for any of the features listed above or suggest new ones.

## Version History

### v0.1.0 (Current)
- Initial release with core functionality
- Authentication, repositories, PRs, branches, pipelines, issues
- Tags, webhooks, deployments, workspaces
- Interactive mode for PR commands
- Shell completion support
- Configuration management
