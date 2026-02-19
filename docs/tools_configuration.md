# Tools Configuration

PicoClaw's tools configuration is located in the `tools` field of `config.json`.

## Directory Structure

```json
{
  "tools": {
    "web": { ... },
    "exec": { ... },
    "cron": { ... },
    "knows": { ... }
  }
}
```

## Web Tools

Web tools are used for web search and fetching.

### Brave

| Config | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | false | Enable Brave search |
| `api_key` | string | - | Brave Search API key |
| `max_results` | int | 5 | Maximum number of results |

### DuckDuckGo

| Config | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | true | Enable DuckDuckGo search |
| `max_results` | int | 5 | Maximum number of results |

### Perplexity

| Config | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | false | Enable Perplexity search |
| `api_key` | string | - | Perplexity API key |
| `max_results` | int | 5 | Maximum number of results |

## Exec Tool

The exec tool is used to execute shell commands.

| Config | Type | Default | Description |
|--------|------|---------|-------------|
| `enable_deny_patterns` | bool | true | Enable default dangerous command blocking |
| `custom_deny_patterns` | array | [] | Custom deny patterns (regular expressions) |

### Functionality

- **`enable_deny_patterns`**: Set to `false` to completely disable the default dangerous command blocking patterns
- **`custom_deny_patterns`**: Add custom deny regex patterns; commands matching these will be blocked

### Default Blocked Command Patterns

By default, PicoClaw blocks the following dangerous commands:

- Delete commands: `rm -rf`, `del /f/q`, `rmdir /s`
- Disk operations: `format`, `mkfs`, `diskpart`, `dd if=`, writing to `/dev/sd*`
- System operations: `shutdown`, `reboot`, `poweroff`
- Command substitution: `$()`, `${}`, backticks
- Pipe to shell: `| sh`, `| bash`
- Privilege escalation: `sudo`, `chmod`, `chown`
- Process control: `pkill`, `killall`, `kill -9`
- Remote operations: `curl | sh`, `wget | sh`, `ssh`
- Package management: `apt`, `yum`, `dnf`, `npm install -g`, `pip install --user`
- Containers: `docker run`, `docker exec`
- Git: `git push`, `git force`
- Other: `eval`, `source *.sh`

### Configuration Example

```json
{
  "tools": {
    "exec": {
      "enable_deny_patterns": true,
      "custom_deny_patterns": [
        "\\brm\\s+-r\\b",
        "\\bkillall\\s+python"
      ],
    }
  }
}
```

## Approval Tool

The approval tool controls permissions for dangerous operations.

| Config | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | true | Enable approval functionality |
| `write_file` | bool | true | Require approval for file writes |
| `edit_file` | bool | true | Require approval for file edits |
| `append_file` | bool | true | Require approval for file appends |
| `exec` | bool | true | Require approval for command execution |
| `timeout_minutes` | int | 5 | Approval timeout in minutes |

## Cron Tool

The cron tool is used for scheduling periodic tasks.

| Config | Type | Default | Description |
|--------|------|---------|-------------|
| `exec_timeout_minutes` | int | 5 | Execution timeout in minutes, 0 means no limit |

## KnowS Toolset

The KnowS toolset provides oncology evidence retrieval and Q&A capabilities via the KnowS API.

### Configuration

| Config | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | false | Enable KnowS tools registration |
| `api_key` | string | - | KnowS API key (`x-api-key`) |
| `api_base_url` | string | - | KnowS API base URL, e.g. `https://dev-api.nullht.com` |
| `default_data_scope` | string[] | `["PAPER","PAPER_CN","GUIDE","MEETING"]` | Default scope for `knows_ai_search` |
| `request_timeout_seconds` | int | 120 | HTTP request timeout |
| `max_retries` | int | 3 | Retry count for retryable failures |
| `retry_backoff_milliseconds` | int | 500 | Exponential retry base backoff |
| `batch_concurrency` | int | 5 | Max concurrency for batch KnowS tools |
| `cache_ttl_minutes` | int | 60 | TTL for evidence detail cache |
| `cache_max_entries` | int | 500 | Max in-memory cached evidence records |

### Registered tools

- `knows_ai_search`
- `knows_answer`
- `knows_batch_answer`
- `knows_evidence_summary`
- `knows_evidence_highlight`
- `knows_get_paper_en`
- `knows_get_paper_cn`
- `knows_get_guide`
- `knows_get_meeting`
- `knows_auto_tagging`
- `knows_list_question`
- `knows_list_interpretation`
- `knows_batch_get_evidence_details`

## Environment Variables

All configuration options can be overridden via environment variables with the format `PICOCLAW_TOOLS_<SECTION>_<KEY>`:

For example:
- `PICOCLAW_TOOLS_WEB_BRAVE_ENABLED=true`
- `PICOCLAW_TOOLS_EXEC_ENABLE_DENY_PATTERNS=false`
- `PICOCLAW_TOOLS_CRON_EXEC_TIMEOUT_MINUTES=10`
- `PICOCLAW_TOOLS_KNOWS_ENABLED=true`
- `PICOCLAW_TOOLS_KNOWS_API_KEY=your_key`
- `PICOCLAW_TOOLS_KNOWS_API_BASE_URL=https://dev-api.nullht.com`

Note: Array-type environment variables are not currently supported and must be set via the config file.
