# Role and Permission Test Suite

This directory contains comprehensive Playwright E2E tests for the Cornerstone role and permission system.

## Overview

The test suite validates the complete role and permission functionality including:

- **Organization Role Management**: Owner, admin, editor, and viewer roles
- **Database Permissions**: Sharing databases and assigning user roles
- **Field-Level Permissions**: Granular read/write/delete permissions per field per role
- **Permission Enforcement**: Ensuring users can only perform allowed actions

## Test Structure

```
e2e/
├── utils/
│   ├── auth.ts              # Authentication helpers (login, logout, register)
│   ├── organization.ts      # Organization management utilities
│   ├── database.ts          # Database permission utilities
│   └── field-permission.ts  # Field-level permission utilities
├── organization-roles.spec.ts      # Organization role tests
├── database-permissions.spec.ts    # Database permission tests
├── field-permissions.spec.ts       # Field permission management tests
├── permission-enforcement.spec.ts  # Permission validation tests
├── test-runner.js           # Test execution script
└── README.md               # This file
```

## Test Files

### 1. Organization Role Tests (`organization-roles.spec.ts`)
- Owner can create organizations and become owner
- Owner can add members with different roles (admin, editor, viewer)
- Owner can update member roles
- Owner can remove members
- Admin can add members
- Viewer cannot add members

### 2. Database Permission Tests (`database-permissions.spec.ts`)
- Owner can create databases
- Owner can share databases with different roles
- Owner can update user roles
- Owner can remove users
- Admin can share databases
- Editor/Viewer cannot share databases

### 3. Field Permission Tests (`field-permissions.spec.ts`)
- Owner can create tables with fields
- Owner can access field permissions page
- Owner can apply permission templates (default, viewer-only, strict)
- Owner can batch select permissions (read/write/delete)
- Owner can set specific field permissions per role
- Owner can reset permissions to default
- Owner cannot edit owner/admin permissions

### 4. Permission Enforcement Tests (`permission-enforcement.spec.ts`)
- Editor can read/write allowed fields
- Editor cannot write to read-only fields
- Viewer can only read allowed fields
- Owner has full access to all fields
- Admin has full access to all fields
- Users without access cannot view tables
- Users with no field permissions see empty forms

## Utilities

### Authentication (`utils/auth.ts`)
```typescript
login(page, user)                    // Login with user credentials
logout(page)                         // Logout current user
registerUser(page, role)             // Register new user with role
ensureLoggedIn(page, role)           // Ensure user is logged in
```

### Organization (`utils/organization.ts`)
```typescript
createOrganization(page, name, desc) // Create new organization
addOrganizationMember(page, org, user, role) // Add member
updateOrganizationMemberRole(page, org, user, newRole) // Update role
removeOrganizationMember(page, org, user) // Remove member
```

### Database (`utils/database.ts`)
```typescript
createDatabase(page, name, desc)     // Create new database
shareDatabase(page, db, user, role)  // Share with user
updateDatabaseUserRole(page, db, user, newRole) // Update role
removeDatabaseUser(page, db, user)   // Remove user
```

### Field Permissions (`utils/field-permission.ts`)
```typescript
createTableWithFields(page, db, table, fields) // Create table with fields
setFieldPermission(page, table, field, role, perms) // Set permissions
applyPermissionTemplate(page, table, template) // Apply template
batchSelectPermissions(page, table, type) // Batch select permissions
navigateToFieldPermissions(page, table) // Navigate to permissions page
```

## Running Tests

### Prerequisites
1. Backend server running on port 8080
2. Frontend dev server running on port 5173
3. Playwright installed in frontend directory

### Method 1: Using Test Runner (Recommended)
```bash
cd frontend/e2e

# List available tests
node test-runner.js list

# Run all tests
node test-runner.js run all

# Run specific test
node test-runner.js run organization-roles.spec.ts

# Show help
node test-runner.js help
```

### Method 2: Direct Playwright Commands
```bash
cd frontend

# Run all tests
npx playwright test e2e/

# Run specific test
npx playwright test e2e/organization-roles.spec.ts

# Run with UI
npx playwright test --ui

# Run headed (visible browser)
npx playwright test --headed
```

### Method 3: Using npm scripts
Add to `frontend/package.json`:
```json
{
  "scripts": {
    "test:roles": "playwright test e2e/organization-roles.spec.ts",
    "test:permissions": "playwright test e2e/database-permissions.spec.ts",
    "test:fields": "playwright test e2e/field-permissions.spec.ts",
    "test:enforcement": "playwright test e2e/permission-enforcement.spec.ts",
    "test:all": "playwright test e2e/",
    "test:runner": "node e2e/test-runner.js"
  }
}
```

## Test Data

The tests use predefined user accounts:
- `test_owner` / `owner@test.com` (password: password123)
- `test_admin` / `admin@test.com` (password: password123)
- `test_editor` / `editor@test.com` (password: password123)
- `test_viewer` / `viewer@test.com` (password: password123)

Tests automatically register these users if they don't exist.

## Test Reports

Playwright generates HTML reports by default:
```bash
# After running tests, open report
npx playwright show-report
```

Reports are stored in `frontend/playwright-report/`

## Debugging

### Run with visible browser
```bash
npx playwright test --headed
```

### Run with slow motion
```bash
npx playwright test --headed --project=chromium --repeat-each=1 --retries=0 --workers=1 --reporter=list --timeout=60000
```

### Debug specific test
```bash
npx playwright test --debug organization-roles.spec.ts
```

### Take screenshots on failure
Tests are configured to capture traces on first retry. Check `playwright.config.ts` for details.

## CI/CD Integration

For CI environments, use:
```bash
# Set CI environment variable
export CI=true

# Run tests (will use headless mode)
npx playwright test e2e/

# Generate JUnit report for CI
npx playwright test --reporter=junit
```

## Troubleshooting

### Tests fail with "Backend server not running"
Start the backend server:
```bash
cd backend/cmd/server
go run main.go
```

### Tests fail with "Frontend server not running"
Start the frontend dev server:
```bash
cd frontend
npm run dev
```

### Tests fail with authentication errors
Ensure the test users are properly registered or clear localStorage and retry.

### Tests timeout
Increase timeout in `playwright.config.ts` or individual test files.

## Best Practices

1. **Always clean up**: Tests should be independent and clean up after themselves
2. **Use descriptive names**: Test names should clearly describe what they test
3. **Check permissions**: Always verify that permissions are actually enforced
4. **Test edge cases**: Include tests for permission denial scenarios
5. **Use utilities**: Leverage the utility functions to avoid code duplication

## Contributing

When adding new tests:
1. Follow the existing file structure
2. Use the utility functions when possible
3. Add tests to the appropriate file or create a new one
4. Update this README with new test descriptions
5. Ensure tests are independent and can run in any order

## License

This test suite is part of the Cornerstone project and follows the same license.