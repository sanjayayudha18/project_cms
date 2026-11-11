/**
 * Integration test for Docker build and nginx serving.
 *
 * This test verifies:
 * - Dockerfile builds successfully (multi-stage: pnpm build → nginx:alpine)
 * - nginx serves index.html on root path (HTTP 200)
 * - Client-side routes return 200 (not 404) via try_files fallback
 *
 * Requirements traced: 8.3, 8.5
 *
 * Prerequisites:
 * - Docker daemon must be running
 * - Port 3001 must be available
 *
 * Run manually: pnpm vitest --run e2e/docker-build.spec.ts
 */

import { execSync } from 'node:child_process'
import { describe, it, expect, beforeAll, afterAll } from 'vitest'

const IMAGE_NAME = 'cms-frontend-test'
const CONTAINER_NAME = 'cms-frontend-test'
const HOST_PORT = 3001
const BASE_URL = `http://localhost:${HOST_PORT}`

function isDockerAvailable(): boolean {
  try {
    execSync('docker info', { stdio: 'ignore', timeout: 5000 })
    return true
  } catch {
    return false
  }
}

function isPortAvailable(port: number): boolean {
  try {
    execSync(
      `powershell -Command "if (Get-NetTCPConnection -LocalPort ${port} -ErrorAction SilentlyContinue) { exit 1 } else { exit 0 }"`,
      { stdio: 'ignore', timeout: 5000 },
    )
    return true
  } catch {
    // Fallback: try curl-based check (cross-platform)
    try {
      execSync(`curl -s -o /dev/null -w "%{http_code}" ${BASE_URL}`, {
        stdio: 'ignore',
        timeout: 3000,
      })
      return false // something is already listening
    } catch {
      return true // nothing listening — port is free
    }
  }
}

function waitForContainer(url: string, maxAttempts = 30, intervalMs = 1000): void {
  for (let i = 0; i < maxAttempts; i++) {
    try {
      const result = execSync(`curl -s -o /dev/null -w "%{http_code}" ${url}`, {
        encoding: 'utf-8',
        timeout: 5000,
      }).trim()
      if (result === '200') return
    } catch {
      // not ready yet
    }
    execSync(`powershell -Command "Start-Sleep -Milliseconds ${intervalMs}"`, {
      stdio: 'ignore',
    })
  }
  throw new Error(`Container did not become healthy at ${url} within ${maxAttempts}s`)
}

function getHttpStatus(url: string): string {
  try {
    return execSync(`curl -s -o /dev/null -w "%{http_code}" ${url}`, {
      encoding: 'utf-8',
      timeout: 10000,
    }).trim()
  } catch {
    return '000'
  }
}

function cleanup(): void {
  try {
    execSync(`docker stop ${CONTAINER_NAME}`, { stdio: 'ignore', timeout: 10000 })
  } catch {
    // container may not be running
  }
  try {
    execSync(`docker rm ${CONTAINER_NAME}`, { stdio: 'ignore', timeout: 10000 })
  } catch {
    // container may not exist
  }
}

const dockerAvailable = isDockerAvailable()

describe.skipIf(!dockerAvailable)('Docker Build Integration', () => {
  beforeAll(() => {
    // Clean up any leftover containers from previous runs
    cleanup()

    if (!isPortAvailable(HOST_PORT)) {
      throw new Error(
        `Port ${HOST_PORT} is already in use. Free it before running Docker integration tests.`,
      )
    }

    // Build the Docker image
    execSync(`docker build -t ${IMAGE_NAME} .`, {
      stdio: 'inherit',
      timeout: 300_000, // 5 min for build
      cwd: process.cwd(),
    })

    // Start the container
    execSync(
      `docker run -d -p ${HOST_PORT}:80 --name ${CONTAINER_NAME} ${IMAGE_NAME}`,
      { stdio: 'inherit', timeout: 30_000 },
    )

    // Wait for the container to be ready
    waitForContainer(BASE_URL)
  }, 360_000) // 6 min timeout for beforeAll

  afterAll(() => {
    cleanup()
  })

  it('should build the Docker image successfully', () => {
    // If we reach this point, the image built successfully in beforeAll
    const result = execSync(`docker image inspect ${IMAGE_NAME}`, {
      encoding: 'utf-8',
      timeout: 10000,
    })
    expect(result).toContain(IMAGE_NAME)
  })

  it('should serve index.html on root path with HTTP 200', () => {
    const status = getHttpStatus(`${BASE_URL}/`)
    expect(status).toBe('200')
  })

  it('should return 200 for /reports route (client-side routing)', () => {
    const status = getHttpStatus(`${BASE_URL}/reports`)
    expect(status).toBe('200')
  })

  it('should return 200 for /forecast route (client-side routing)', () => {
    const status = getHttpStatus(`${BASE_URL}/forecast`)
    expect(status).toBe('200')
  })

  it('should return 200 for /replenishment route (client-side routing)', () => {
    const status = getHttpStatus(`${BASE_URL}/replenishment`)
    expect(status).toBe('200')
  })

  it('should return 200 for /invoices route (client-side routing)', () => {
    const status = getHttpStatus(`${BASE_URL}/invoices`)
    expect(status).toBe('200')
  })

  it('should return 200 for unknown routes (SPA fallback)', () => {
    const status = getHttpStatus(`${BASE_URL}/some/unknown/path`)
    expect(status).toBe('200')
  })

  it('should serve HTML content (not empty response)', () => {
    const body = execSync(`curl -s ${BASE_URL}/`, {
      encoding: 'utf-8',
      timeout: 10000,
    })
    expect(body).toContain('<!DOCTYPE html')
    expect(body).toContain('<div id="root"')
  })
})
