# Homeport Environment

You are running inside Homeport, a self-hosted remote development environment. You have access to the `homeport` CLI to manage repositories and dev servers.

## Homeport CLI Commands

### Repository Management

```bash
# List all cloned repositories
homeport repos

# Clone a new repository from GitHub
homeport clone owner/repo
# Example: homeport clone facebook/react
```

### Dev Server Management

```bash
# Start the dev server for a repository
homeport start <repo-name>

# Stop the dev server for a repository
homeport stop <repo-name>

# View dev server logs
homeport logs <repo-name>
homeport logs <repo-name> -f        # Follow logs
homeport logs <repo-name> -n 100    # Show last 100 lines
```

### Port Management

```bash
# List all active ports
homeport list

# Get the external URL for a port
homeport url <port>

# Share a port externally
homeport share <port>              # Private (requires Homeport login)
homeport share <port> --public     # Public (anyone can access)
homeport share <port> --password   # Password protected

# Remove sharing from a port
homeport unshare <port>
```

### URLs

```bash
# Get VS Code URL for a repository
homeport open <repo-name>

# Get terminal URL for a repository
homeport terminal <repo-name>
```

### System

```bash
# Check Homeport status
homeport status
```

## Common Workflows

### Start working on a project
```bash
homeport clone owner/repo
homeport start repo-name
homeport list  # See which port it's running on
```

### Share your work
```bash
homeport list                      # Find the port
homeport share 3000 --public       # Make it public
homeport url 3000                  # Get the URL to share
```

### Debug a failing dev server
```bash
homeport logs repo-name -f         # Watch logs in real-time
homeport stop repo-name            # Stop if needed
homeport start repo-name           # Restart
```

## Notes

- Repository names are case-insensitive
- You can use either the repo name or ID (shown in `homeport repos`)
- Dev servers must have a `start_command` configured in Homeport to use `start`/`stop`
- All repositories are stored in `/home/coder/repos/`
