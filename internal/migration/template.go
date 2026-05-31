package migration

const DefaultTemplate = `source:
  type: sqlite
  dsn: ./source.db

target:
  database_name: ""

tables:
  include: []
  exclude:
    - schema_migrations
  rename: {}

data:
  enabled: true
  batch_size: 500
  pagination_strategy: cursor
  cursor_column: ""
  filters: {}
  max_concurrent_tables: 1

mapping:
  overrides: {}

options:
  dry_run: false
  continue_on_error: false
  log_level: info
  validate_after: true
  checkpoint_interval: 100
  rollback_on_failure: table
`
