import path from 'path';
import fs from 'fs';
import cp from 'child_process';

export const DEFAULT_BASE_URL = process.env.BASE_URL || `http://localhost:${process.env.TEST_PORT || 8080}`;

export async function waitAvailable(baseUrl = DEFAULT_BASE_URL, timeoutMs = 120000, intervalMs = 500) {
    const startTime = Date.now();
    while (true) {
        try {
            const response = await fetch(`${baseUrl}/healthz`);
            if (response.ok) {
                return;
            }
        } catch (err) {
            // Ignore errors and retry
        }
        if (Date.now() - startTime > timeoutMs) {
            throw new Error(`Service at ${baseUrl} did not become available within ${timeoutMs} ms`);
        }
        await new Promise(resolve => setTimeout(resolve, intervalMs));
    }
}

export async function launchSnapshot(snapshotName) {
    const snapshotsDir = path.resolve(process.cwd(), 'snapshots', snapshotName);
    if (!fs.existsSync(snapshotsDir)) {
        throw new Error(`Snapshot directory does not exist: ${snapshotsDir}`);
    }

    const snapshotDockerCompose = path.join(snapshotsDir, 'docker-compose.yaml');
    if (!fs.existsSync(snapshotDockerCompose)) {
        throw new Error(`docker-compose.yaml not found in snapshot directory: ${snapshotDockerCompose}`);
    }

    return new Promise((resolve, reject) => {
        try {
            let stderr = '';
            const dockerComposeProcess = cp.spawn('docker', ['compose', '-f', snapshotDockerCompose, 'up', '-d', '--build'], {
                cwd: snapshotsDir,
                env: process.env,
            });
            dockerComposeProcess.stderr?.on('data', (d) => {
                stderr += d.toString();
            });
            dockerComposeProcess.on('close', (code) => {
                if (code === 0) {
                    resolve();
                } else {
                    reject(new Error(`docker-compose up exited with code ${code}: ${stderr}`));
                }
            });
        } catch (err) {
            reject(err);
        }
    });
}

export async function teardownSnapshot(snapshotName) {
    const snapshotsDir = path.resolve(process.cwd(), 'snapshots', snapshotName);
    const snapshotDockerCompose = path.join(snapshotsDir, 'docker-compose.yaml');

    return new Promise((resolve, reject) => {
        try {
            let stderr = '';
            const dockerComposeProcess = cp.spawn('docker', ['compose', '-f', snapshotDockerCompose, 'down', '-v'], {
                cwd: snapshotsDir,
                env: process.env,
            });
            dockerComposeProcess.stderr?.on('data', (d) => {
                stderr += d.toString();
            });
            dockerComposeProcess.on('close', (code) => {
                if (code === 0) {
                    resolve();
                } else {
                    reject(new Error(`docker-compose down exited with code ${code}: ${stderr}`));
                }
            });
        } catch (err) {
            reject(err);
        }
    });
}

/**
 * Parse OpenAI Chat Completions SSE stream (`data: {...}\n\n` and `data: [DONE]\n\n`)
 */
export function parseChatSSE(rawText) {
    const chunks = [];
    let done = false;
    const lines = rawText.split(/\r?\n/);
    for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed) continue;
        if (trimmed.startsWith('data:')) {
            const payload = trimmed.slice('data:'.length).trim();
            if (payload === '[DONE]') {
                done = true;
            } else {
                chunks.push(JSON.parse(payload));
            }
        }
    }
    return { chunks, done };
}

/**
 * Parse OpenAI Responses API semantic SSE stream (`event: <type>\ndata: <json>\n\n` and `data: [DONE]\n\n`)
 */
export function parseResponsesSSE(rawText) {
    const events = [];
    let done = false;
    const blocks = rawText.split(/\r?\n\r?\n/);
    for (const block of blocks) {
        const trimmed = block.trim();
        if (!trimmed) continue;
        const lines = trimmed.split(/\r?\n/);
        let eventName = null;
        let dataPayload = null;
        for (const line of lines) {
            if (line.startsWith('event:')) {
                eventName = line.slice('event:'.length).trim();
            } else if (line.startsWith('data:')) {
                dataPayload = line.slice('data:'.length).trim();
            }
        }
        if (dataPayload === '[DONE]') {
            done = true;
        } else if (dataPayload) {
            const parsed = JSON.parse(dataPayload);
            events.push({
                event: eventName || parsed.type,
                data: parsed,
            });
        }
    }
    return { events, done };
}

/**
 * Parse Vertex AI SSE stream (`data: <GenerateContentResponse JSON>\n\n`)
 */
export function parseVertexSSE(rawText) {
    const chunks = [];
    const lines = rawText.split(/\r?\n/);
    for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed) continue;
        if (trimmed.startsWith('data:')) {
            const payload = trimmed.slice('data:'.length).trim();
            if (payload && payload !== '[DONE]') {
                chunks.push(JSON.parse(payload));
            }
        }
    }
    return { chunks };
}

export class AiProviderClient {
    constructor(baseUrl = DEFAULT_BASE_URL) {
        this.baseUrl = baseUrl;
    }

    // Health check
    async healthz() {
        const response = await fetch(`${this.baseUrl}/healthz`);
        if (!response.ok) {
            throw new Error(`Health check failed with status ${response.status}`);
        }
        return await response.json();
    }

    // Models API
    async listModels(headers = {}) {
        const response = await fetch(`${this.baseUrl}/v1/models`, { headers });
        if (!response.ok) {
            throw new Error(`List models failed with status ${response.status}`);
        }
        return await response.json();
    }

    async getModel(modelId, headers = {}) {
        const response = await fetch(`${this.baseUrl}/v1/models/${encodeURIComponent(modelId)}`, { headers });
        return response;
    }

    // Chat Completions API
    async createChatCompletion(body, headers = {}) {
        const response = await fetch(`${this.baseUrl}/v1/chat/completions`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                ...headers,
            },
            body: typeof body === 'string' ? body : JSON.stringify(body),
        });
        return response;
    }

    // Responses API
    async createResponse(body, headers = {}) {
        const response = await fetch(`${this.baseUrl}/v1/responses`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                ...headers,
            },
            body: typeof body === 'string' ? body : JSON.stringify(body),
        });
        return response;
    }

    // Vertex AI / Gemini API (v1beta)
    async vertexGenerateContent(model, body, headers = {}, query = '') {
        const qs = query ? (query.startsWith('?') ? query : `?${query}`) : '';
        const response = await fetch(`${this.baseUrl}/v1beta/models/${encodeURIComponent(model)}:generateContent${qs}`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                ...headers,
            },
            body: typeof body === 'string' ? body : JSON.stringify(body),
        });
        return response;
    }

    async vertexStreamGenerateContent(model, body, headers = {}, query = '') {
        const qs = query ? (query.startsWith('?') ? query : `?${query}`) : '';
        const response = await fetch(`${this.baseUrl}/v1beta/models/${encodeURIComponent(model)}:streamGenerateContent${qs}`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                ...headers,
            },
            body: typeof body === 'string' ? body : JSON.stringify(body),
        });
        return response;
    }

    // Vertex AI / Gemini API (v1 projects/locations/publishers/google)
    async vertexProjectGenerateContent(project, location, model, body, headers = {}, query = '') {
        const qs = query ? (query.startsWith('?') ? query : `?${query}`) : '';
        const url = `${this.baseUrl}/v1/projects/${encodeURIComponent(project)}/locations/${encodeURIComponent(location)}/publishers/google/models/${encodeURIComponent(model)}:generateContent${qs}`;
        const response = await fetch(url, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                ...headers,
            },
            body: typeof body === 'string' ? body : JSON.stringify(body),
        });
        return response;
    }

    async vertexProjectStreamGenerateContent(project, location, model, body, headers = {}, query = '') {
        const qs = query ? (query.startsWith('?') ? query : `?${query}`) : '';
        const url = `${this.baseUrl}/v1/projects/${encodeURIComponent(project)}/locations/${encodeURIComponent(location)}/publishers/google/models/${encodeURIComponent(model)}:streamGenerateContent${qs}`;
        const response = await fetch(url, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                ...headers,
            },
            body: typeof body === 'string' ? body : JSON.stringify(body),
        });
        return response;
    }

    // Testing API
    async getRecordedRequests(params = {}) {
        const qs = new URLSearchParams(params).toString();
        const url = qs ? `${this.baseUrl}/testing/requests?${qs}` : `${this.baseUrl}/testing/requests`;
        const response = await fetch(url);
        if (!response.ok) {
            throw new Error(`Get recorded requests failed with status ${response.status}`);
        }
        return await response.json();
    }

    async getRecordedRequestById(id) {
        return await fetch(`${this.baseUrl}/testing/requests/${encodeURIComponent(id)}`);
    }

    async clearRecordedRequests() {
        const response = await fetch(`${this.baseUrl}/testing/requests`, { method: 'DELETE' });
        if (!response.ok) {
            throw new Error(`Clear recorded requests failed with status ${response.status}`);
        }
        return await response.json();
    }

    async listMatchers() {
        const response = await fetch(`${this.baseUrl}/testing/matchers`);
        if (!response.ok) {
            throw new Error(`List matchers failed with status ${response.status}`);
        }
        return await response.json();
    }

    async getMatcher(id) {
        return await fetch(`${this.baseUrl}/testing/matchers/${encodeURIComponent(id)}`);
    }

    async putMatcher(id, body) {
        return await fetch(`${this.baseUrl}/testing/matchers/${encodeURIComponent(id)}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: typeof body === 'string' ? body : JSON.stringify(body),
        });
    }

    async deleteMatcher(id) {
        return await fetch(`${this.baseUrl}/testing/matchers/${encodeURIComponent(id)}`, {
            method: 'DELETE',
        });
    }

    async putModel(id, body) {
        return await fetch(`${this.baseUrl}/testing/models/${encodeURIComponent(id)}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: typeof body === 'string' ? body : JSON.stringify(body),
        });
    }

    async deleteModel(id) {
        return await fetch(`${this.baseUrl}/testing/models/${encodeURIComponent(id)}`, {
            method: 'DELETE',
        });
    }

    async getConfig() {
        const response = await fetch(`${this.baseUrl}/testing/config`);
        if (!response.ok) {
            throw new Error(`Get config failed with status ${response.status}`);
        }
        return await response.json();
    }

    async patchConfig(body) {
        return await fetch(`${this.baseUrl}/testing/config`, {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: typeof body === 'string' ? body : JSON.stringify(body),
        });
    }

    async resetState() {
        const response = await fetch(`${this.baseUrl}/testing/reset`, { method: 'POST' });
        if (!response.ok) {
            throw new Error(`Reset state failed with status ${response.status}`);
        }
        return await response.json();
    }
}
