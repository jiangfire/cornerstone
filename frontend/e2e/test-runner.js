#!/usr/bin/env node

/**
 * Role and Permission Test Runner
 *
 * This script provides a convenient way to run all role and permission tests
 * with proper setup and teardown.
 */

// eslint-disable-next-line @typescript-eslint/no-require-imports
const { spawn } = require('child_process');
// eslint-disable-next-line @typescript-eslint/no-require-imports
const fs = require('fs');
// eslint-disable-next-line @typescript-eslint/no-require-imports
const path = require('path');

// Test configuration
const TEST_CONFIG = {
  frontendDir: path.join(__dirname, '..', 'frontend'),
  testDir: path.join(__dirname, '..', 'frontend', 'e2e'),
  timeout: 300000, // 5 minutes
};

// Test files to run (in order of dependency)
const TEST_FILES = [
  'organization-roles.spec.ts',
  'database-permissions.spec.ts',
  'field-permissions.spec.ts',
  'permission-enforcement.spec.ts'
];

// Colors for console output
const colors = {
  reset: '\x1b[0m',
  red: '\x1b[31m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  magenta: '\x1b[35m',
  cyan: '\x1b[36m',
  gray: '\x1b[90m',
};

function log(message, color = 'reset') {
  console.log(`${colors[color]}${message}${colors.reset}`);
}

function error(message) {
  log(`❌ ${message}`, 'red');
}

function success(message) {
  log(`✅ ${message}`, 'green');
}

function info(message) {
  log(`ℹ️  ${message}`, 'blue');
}

function warn(message) {
  log(`⚠️  ${message}`, 'yellow');
}

/**
 * Check if all required files exist
 */
function checkTestFiles() {
  info('Checking test files...');

  const missingFiles = [];
  const existingFiles = [];

  for (const file of TEST_FILES) {
    const filePath = path.join(TEST_CONFIG.testDir, file);
    if (fs.existsSync(filePath)) {
      existingFiles.push(file);
    } else {
      missingFiles.push(file);
    }
  }

  // Check utils
  const utils = ['auth.ts', 'organization.ts', 'database.ts', 'field-permission.ts'];
  for (const util of utils) {
    const utilPath = path.join(TEST_CONFIG.testDir, 'utils', util);
    if (!fs.existsSync(utilPath)) {
      missingFiles.push(`utils/${util}`);
    }
  }

  if (missingFiles.length > 0) {
    error(`Missing test files: ${missingFiles.join(', ')}`);
    return false;
  }

  success(`Found ${existingFiles.length} test files and ${utils.length} utility files`);
  return true;
}

/**
 * Check if Playwright is installed
 */
function checkPlaywright() {
  info('Checking Playwright installation...');

   
  const packageJsonPath = path.join(TEST_CONFIG.frontendDir, 'package.json');
  if (!fs.existsSync(packageJsonPath)) {
    error('package.json not found in frontend directory');
    return false;
  }

  const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, 'utf8'));
  const hasPlaywright = packageJson.devDependencies && (
    packageJson.devDependencies['@playwright/test'] ||
    packageJson.devDependencies['playwright']
  );

  if (!hasPlaywright) {
    error('Playwright not found in devDependencies');
    warn('Run: cd frontend && npm install --save-dev @playwright/test');
    return false;
  }

  success('Playwright is installed');
  return true;
}

/**
 * Check if backend server is running
 */
async function checkBackendServer() {
  info('Checking backend server...');

  try {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const http = require('http');
    return new Promise((resolve) => {
      const req = http.get('http://localhost:8080/api/health', (res) => {
        if (res.statusCode === 200) {
          success('Backend server is running on port 8080');
          resolve(true);
        } else {
          warn('Backend server responded but with non-200 status');
          resolve(true);
        }
      });

      req.on('error', () => {
        warn('Backend server not running on port 8080');
        warn('Some tests may fail if they require backend API');
        warn('Start backend with: cd backend/cmd/server && go run main.go');
        resolve(false);
      });

      req.setTimeout(3000, () => {
        req.destroy();
        warn('Backend server check timeout');
        resolve(false);
      });
    });
  } catch {
    warn('Could not check backend server');
    return false;
  }
}

/**
 * Run a single test file
 */
function runTestFile(fileName, options = {}) {
  return new Promise((resolve, reject) => {
    const filePath = path.join(TEST_CONFIG.testDir, fileName);
    const cwd = TEST_CONFIG.frontendDir;

    info(`Running: ${fileName}`);

    const args = ['playwright', 'test', filePath];
    if (options.headless !== false) {
      args.push('--headed=false');
    }
    if (options.reporter) {
      args.push('--reporter', options.reporter);
    }

    const proc = spawn('npx', args, {
      cwd,
      stdio: 'inherit',
      env: { ...process.env, CI: 'false' }
    });

    proc.on('close', (code) => {
      if (code === 0) {
        success(`Test ${fileName} passed`);
        resolve(code);
      } else {
        error(`Test ${fileName} failed with code ${code}`);
        reject(new Error(`Test failed: ${fileName}`));
      }
    });

    proc.on('error', (err) => {
      error(`Failed to start test: ${err.message}`);
      reject(err);
    });
  });
}

/**
 * Run all tests
 */
async function runAllTests(options = {}) {
  info('Starting role and permission test suite...');
  info('This will test:');
  info('  - Organization role management (owner, admin, editor, viewer)');
  info('  - Database permission management (sharing, role assignment)');
  info('  - Field-level permission management (R/W/D per role)');
  info('  - Permission enforcement (users can only do what they\'re allowed)');

  // Pre-flight checks
  if (!checkTestFiles()) {
    error('Missing test files. Cannot proceed.');
    return false;
  }

  if (!checkPlaywright()) {
    error('Playwright not properly installed. Cannot proceed.');
    return false;
  }

  await checkBackendServer();

  // Run tests
  const results = [];
  for (const testFile of TEST_FILES) {
    try {
      await runTestFile(testFile, options);
      results.push({ file: testFile, status: 'passed' });
    } catch (err) {
      results.push({ file: testFile, status: 'failed', error: err.message });
      if (!options.continueOnFailure) {
        break;
      }
    }
  }

  // Summary
  log('\n' + '='.repeat(60));
  info('TEST SUMMARY');
  log('='.repeat(60));

  let passed = 0;
  let failed = 0;

  for (const result of results) {
    if (result.status === 'passed') {
      log(`✅ ${result.file}`, 'green');
      passed++;
    } else {
      log(`❌ ${result.file} - ${result.error}`, 'red');
      failed++;
    }
  }

  log('='.repeat(60));
  log(`Total: ${results.length} | Passed: ${passed} | Failed: ${failed}`);

  if (failed === 0) {
    success('🎉 All tests passed!');
    return true;
  } else {
    error('❌ Some tests failed');
    return false;
  }
}

/**
 * Run specific test file
 */
async function runSpecificTest(fileName, options = {}) {
  if (!TEST_FILES.includes(fileName)) {
    error(`Unknown test file: ${fileName}`);
    info(`Available files: ${TEST_FILES.join(', ')}`);
    return false;
  }

  return await runTestFile(fileName, options);
}

/**
 * List available tests
 */
function listTests() {
  info('Available role and permission tests:');
  log('');
  for (const file of TEST_FILES) {
    log(`  - ${file}`);
  }
  log('');
  info('Utility files:');
  log('  - utils/auth.ts (authentication helpers)');
  log('  - utils/organization.ts (organization management)');
  log('  - utils/database.ts (database permissions)');
  log('  - utils/field-permission.ts (field-level permissions)');
}

/**
 * Main CLI interface
 */
async function main() {
  const args = process.argv.slice(2);

  if (args.length === 0) {
    // Run all tests by default
    await runAllTests({ continueOnFailure: true });
    return;
  }

  const command = args[0];

  switch (command) {
    case 'list':
      listTests();
      break;

    case 'run':
      if (args.length < 2) {
        error('Usage: run <test-file>');
        info('Example: run organization-roles.spec.ts');
        info('Or use "all" to run everything');
        return;
      }
      if (args[1] === 'all') {
        await runAllTests({ continueOnFailure: true });
      } else {
        await runSpecificTest(args[1]);
      }
      break;

    case 'help':
    case '--help':
    case '-h':
      showHelp();
      break;

    default:
      error(`Unknown command: ${command}`);
      showHelp();
  }
}

function showHelp() {
  log(`
Role and Permission Test Runner

Usage:
  node test-runner.js [command] [options]

Commands:
  list              List all available test files
  run <file>        Run specific test file
  run all           Run all tests
  help              Show this help

Examples:
  node test-runner.js list
  node test-runner.js run all
  node test-runner.js run organization-roles.spec.ts

Test Files:
  - organization-roles.spec.ts     (Organization member role management)
  - database-permissions.spec.ts   (Database sharing and role assignment)
  - field-permissions.spec.ts      (Field-level R/W/D permissions)
  - permission-enforcement.spec.ts (Permission validation tests)

Utilities:
  - utils/auth.ts                  (Login, logout, registration)
  - utils/organization.ts          (Organization CRUD operations)
  - utils/database.ts              (Database permission operations)
  - utils/field-permission.ts      (Field permission operations)

Note: Make sure backend server is running on port 8080
      and frontend dev server is running on port 5173
  `);
}

// Run if called directly
if (require.main === module) {
  main().catch(err => {
    error(`Fatal error: ${err.message}`);
    process.exit(1);
  });
}

module.exports = {
  runAllTests,
  runSpecificTest,
  listTests,
  checkTestFiles,
  checkPlaywright,
  checkBackendServer,
  TEST_FILES,
};