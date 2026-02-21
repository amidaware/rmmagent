# Tactical RMM Agent Documentation

Welcome to the comprehensive documentation for the Tactical RMM Agent, a powerful cross-platform remote monitoring and management agent written in Go that enables enterprise-grade endpoint management across Windows, macOS, and Linux systems.

## 📚 Table of Contents

### Getting Started
New to the Tactical RMM Agent? Start here for quick setup and initial configuration:

- [Introduction](./getting-started/introduction.md) - Overview of the agent, features, and architecture
- [Prerequisites](./getting-started/prerequisites.md) - System requirements and preparation steps
- [Quick Start Guide](./getting-started/quick-start.md) - 5-minute installation and setup
- [First Steps](./getting-started/first-steps.md) - Initial configuration and feature exploration

### Development
Comprehensive guides for developers contributing to the project:

- [Development Overview](./development/README.md) - Development documentation structure and technology stack
- [Environment Setup](./development/setup/environment.md) - IDE configuration and development tools
- [Local Development](./development/setup/local-development.md) - Clone, build, run, and debug locally
- [Architecture Overview](./development/architecture/README.md) - System design and component relationships
- [Security Best Practices](./development/security/README.md) - Authentication, encryption, and secure coding
- [Testing Guide](./development/testing/README.md) - Test structure, running tests, and coverage requirements
- [Contributing Guidelines](./development/contributing/guidelines.md) - Code style, PR process, and review standards

### Reference Documentation
Technical reference documentation is being generated. Check back soon for detailed API specifications, configuration guides, and service documentation.

### Architecture Diagrams
Visual documentation showing system architecture and data flow:

- **System Architecture**: High-level overview of agent components and server communication
- **Component Relationships**: Detailed component interactions and dependencies  
- **Data Flow Diagrams**: System monitoring check-in flow and remote command execution flow

View all architecture diagrams at: `./docs/diagrams/architecture/`

## 🚀 Quick Links

| Resource | Description |
|----------|-------------|
| [Project README](../README.md) | Main project overview and quick start |
| [Contributing](../CONTRIBUTING.md) | How to contribute to the project |
| [License](../LICENSE.md) | License information and terms |

## 🏗️ Architecture Overview

The Tactical RMM Agent is built with a service-oriented architecture featuring:

- **Agent Core**: Main service process with cross-platform implementations
- **Communication Layer**: HTTPS REST API and NATS messaging for real-time communication
- **System Integration**: Process monitoring, service management, and hardware inventory
- **Security Framework**: Token-based authentication and encrypted communications
- **Platform Support**: Native implementations for Windows, macOS, and Linux

## 🔧 Key Features Documented

### System Monitoring
- Real-time system metrics collection (CPU, memory, disk space)
- Hardware inventory and system information gathering
- Process and service monitoring with health checks
- Custom script execution and validation

### Remote Management  
- Secure remote command execution (PowerShell, Bash, Python)
- Service lifecycle management (start, stop, restart)
- Scheduled task creation and automated maintenance
- File transfer and deployment capabilities

### Enterprise Security
- Token-based authentication with automatic refresh
- Encrypted HTTPS and NATS communication channels
- OpenFrame integration for enhanced enterprise security
- Input validation and sandboxed command execution

### Cross-Platform Support
- Windows: Complete Windows API integration, service management, Windows Update handling
- macOS: launchd service integration, system framework access, native monitoring
- Linux: systemd integration, package management, cross-distro compatibility

## 📋 Documentation Standards

This documentation follows markdown best practices and includes:

- **Code Examples**: Practical examples for all major features and use cases
- **Platform-Specific Guides**: Tailored instructions for Windows, macOS, and Linux
- **Security Guidelines**: Best practices for secure deployment and configuration  
- **Troubleshooting**: Common issues and solutions for setup and operation
- **API References**: Complete API documentation for developers and integrators

## 🤝 Contributing to Documentation

Documentation improvements are welcome! See our [Contributing Guidelines](./development/contributing/guidelines.md) for:

- Documentation style standards
- Review process for documentation changes
- How to add new guides and references
- Testing documentation changes locally

## 📖 Support and Community

- **GitHub Issues**: Report bugs, request features, or ask questions
- **Development Discussions**: Technical discussions about architecture and implementation
- **Security Reports**: Responsible disclosure of security vulnerabilities
- **Community Guidelines**: Respectful collaboration standards

---

*Documentation generated by [OpenFrame Doc Orchestrator](https://github.com/flamingo-stack/openframe-oss-tenant)*

**Last Updated**: This documentation is automatically maintained and updated as the codebase evolves.