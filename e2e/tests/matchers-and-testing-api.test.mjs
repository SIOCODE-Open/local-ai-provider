import { expect } from 'chai';
import {
    AiProviderClient,
    DEFAULT_BASE_URL,
    launchSnapshot,
    teardownSnapshot,
    waitAvailable,
    parseChatSSE,
} from './utils/index.mjs';

describe('matchers-and-testing-api - Matchers, Selection Strategies, Errors, Delays & Testing API', () => {
    const client = new AiProviderClient(DEFAULT_BASE_URL);

    before(async () => {
        await launchSnapshot('simple');
        await waitAvailable(DEFAULT_BASE_URL);
    });

    after(async () => {
        await teardownSnapshot('simple');
    });

    beforeEach(async () => {
        await client.resetState();
    });

    describe('Matcher Evaluation Priority, Tie-Breaking & Disabled Flag', () => {
        it('Should evaluate higher priority matchers before lower priority matchers', async () => {
            await client.putMatcher('low-priority', {
                priority: 10,
                match: { message_contains: 'priority test' },
                response: { text: 'From low priority' },
            });
            await client.putMatcher('high-priority', {
                priority: 200,
                match: { message_contains: 'priority test' },
                response: { text: 'From high priority' },
            });

            const res = await client.createChatCompletion({
                model: 'gpt-4o',
                messages: [{ role: 'user', content: 'Run priority test now' }],
            });
            const body = await res.json();
            expect(body.choices[0].message.content).to.equal('From high priority');
        });

        it('Should skip disabled matchers', async () => {
            await client.putMatcher('disabled-high', {
                priority: 300,
                disabled: true,
                match: { message_contains: 'skip me' },
                response: { text: 'Should not return this' },
            });
            await client.putMatcher('enabled-low', {
                priority: 20,
                disabled: false,
                match: { message_contains: 'skip me' },
                response: { text: 'Returned from enabled matcher' },
            });

            const res = await client.createChatCompletion({
                model: 'gpt-4o',
                messages: [{ role: 'user', content: 'Please skip me' }],
            });
            const body = await res.json();
            expect(body.choices[0].message.content).to.equal('Returned from enabled matcher');
        });
    });

    describe('Comprehensive Matcher Criteria', () => {
        it('Should match message_exact, model_prefix, system_contains, system_regex, and any_message_contains', async () => {
            await client.putMatcher('complex-criteria', {
                priority: 250,
                match: {
                    api_type: 'chat_completions',
                    model_prefix: 'gpt-4',
                    message_exact: 'Confirm order',
                    any_message_contains: 'order #9876',
                    system_contains: 'E-Commerce bot',
                    system_regex: '(?i)helpful\\s+e-commerce',
                    stream: false,
                },
                response: {
                    text: 'Order #9876 confirmed!',
                },
            });

            const res = await client.createChatCompletion({
                model: 'gpt-4o-mini',
                messages: [
                    { role: 'system', content: 'You are a helpful E-Commerce bot.' },
                    { role: 'user', content: 'I want to place order #9876.' },
                    { role: 'assistant', content: 'Would you like to confirm?' },
                    { role: 'user', content: 'Confirm order' },
                ],
                stream: false,
            });
            const body = await res.json();
            expect(body.choices[0].message.content).to.equal('Order #9876 confirmed!');
        });

        it('Should match role-scoped message checks, header_match (case-insensitive), tool_name, tool_choice, tool_call_id, and tool_output_regex', async () => {
            await client.putMatcher('header-and-tool-criteria', {
                priority: 260,
                match: {
                    role: 'assistant',
                    message_contains: 'Fetching DB record',
                    has_tools: true,
                    tool_name: 'query_db',
                    tool_choice: 'required',
                    has_tool_output: true,
                    tool_call_id: 'call_db_42',
                    tool_output_regex: '"rows":\\s*\\[1,\\s*2,\\s*3\\]',
                    header_match: {
                        'x-custom-trace': 'trace-abc-123',
                    },
                },
                response: {
                    text: 'Found 3 rows in DB.',
                },
            });

            const res = await client.createChatCompletion(
                {
                    model: 'gpt-4o',
                    messages: [
                        { role: 'user', content: 'Check the DB' },
                        {
                            role: 'assistant',
                            content: 'Fetching DB record...',
                            tool_calls: [
                                {
                                    id: 'call_db_42',
                                    type: 'function',
                                    function: { name: 'query_db', arguments: '{}' },
                                },
                            ],
                        },
                        {
                            role: 'tool',
                            tool_call_id: 'call_db_42',
                            content: '{"rows": [1, 2, 3]}',
                        },
                    ],
                    tools: [
                        {
                            type: 'function',
                            function: { name: 'query_db', parameters: { type: 'object' } },
                        },
                    ],
                    tool_choice: 'required',
                },
                {
                    'X-Custom-Trace': 'trace-abc-123',
                }
            );
            expect(res.status).to.equal(200);
            const body = await res.json();
            expect(body.choices[0].message.content).to.equal('Found 3 rows in DB.');
        });
    });

    describe('Selection Strategies & Fallback Default Responses', () => {
        it('Should cycle through responses in sequential strategy and wrap around', async () => {
            // greeting-rule in simple snapshot has 2 sequential responses
            const r1 = await (await client.createChatCompletion({
                model: 'gpt-4o',
                messages: [{ role: 'user', content: 'hello' }],
            })).json();
            const r2 = await (await client.createChatCompletion({
                model: 'gpt-4o',
                messages: [{ role: 'user', content: 'hi' }],
            })).json();
            const r3 = await (await client.createChatCompletion({
                model: 'gpt-4o',
                messages: [{ role: 'user', content: 'hey' }],
            })).json();

            expect(r1.choices[0].message.content).to.equal('Hello! How can I assist you today?');
            expect(r2.choices[0].message.content).to.equal('Hi there! What can I help you build?');
            expect(r3.choices[0].message.content).to.equal('Hello! How can I assist you today?');
        });

        it('Should support "first" and "random" selection strategies', async () => {
            await client.putMatcher('first-strategy-rule', {
                priority: 200,
                match: { message_contains: 'test first strategy' },
                response: {
                    selection_strategy: 'first',
                    responses: ['First Option', 'Second Option', 'Third Option'],
                },
            });

            for (let i = 0; i < 3; i++) {
                const body = await (await client.createChatCompletion({
                    model: 'gpt-4o',
                    messages: [{ role: 'user', content: 'test first strategy' }],
                })).json();
                expect(body.choices[0].message.content).to.equal('First Option');
            }

            const candidates = ['Random A', 'Random B', 'Random C'];
            await client.putMatcher('random-strategy-rule', {
                priority: 200,
                match: { message_contains: 'test random strategy' },
                response: {
                    selection_strategy: 'random',
                    responses: candidates,
                },
            });

            for (let i = 0; i < 5; i++) {
                const body = await (await client.createChatCompletion({
                    model: 'gpt-4o',
                    messages: [{ role: 'user', content: 'test random strategy' }],
                })).json();
                expect(candidates).to.include(body.choices[0].message.content);
            }
        });

        it('Should fall back to default_response and record matched_rule_id as "__default__"', async () => {
            const res = await client.createChatCompletion({
                model: 'gpt-4o',
                messages: [{ role: 'user', content: 'An unmatched prompt xyz123' }],
            });
            expect(res.status).to.equal(200);
            const body = await res.json();
            expect(body.choices[0].message.content).to.equal('I am a local AI provider default response.');

            const recorded = await client.getRecordedRequests({ matched_rule_id: '__default__' });
            expect(recorded.count).to.equal(1);
            expect(recorded.requests[0].matched_rule_id).to.equal('__default__');
        });
    });

    describe('Simulated Errors & Artificial Delays', () => {
        it('Should return rule-level simulated HTTP error (429 / 500)', async () => {
            await client.putMatcher('rate-limit-rule', {
                priority: 300,
                match: { message_contains: 'trigger 429' },
                response: {
                    status_code: 429,
                    error_message: 'Too many requests for model gpt-4o',
                    error_type: 'rate_limit_error',
                    error_code: 'rate_limit_exceeded',
                },
            });

            const res = await client.createChatCompletion({
                model: 'gpt-4o',
                messages: [{ role: 'user', content: 'Please trigger 429 error' }],
            });
            expect(res.status).to.equal(429);
            const errBody = await res.json();
            expect(errBody).to.have.property('error');
            expect(errBody.error.message).to.equal('Too many requests for model gpt-4o');
            expect(errBody.error.type).to.equal('rate_limit_error');
            expect(errBody.error.code).to.equal('rate_limit_exceeded');
        });

        it('Should return global simulated error when enabled via PATCH /testing/config', async () => {
            const patchRes = await client.patchConfig({
                simulated_error: {
                    enabled: true,
                    status_code: 503,
                    message: 'Service temporarily overloaded',
                    type: 'server_error',
                    code: 'overloaded',
                },
            });
            expect(patchRes.status).to.equal(200);

            const resChat = await client.createChatCompletion({
                model: 'gpt-4o',
                messages: [{ role: 'user', content: 'hello' }],
            });
            expect(resChat.status).to.equal(503);

            const resResp = await client.createResponse({
                model: 'gpt-4o',
                input: 'hello',
            });
            expect(resResp.status).to.equal(503);

            const resVertex = await client.vertexGenerateContent('gemini-1.5-pro', {
                contents: [{ role: 'user', parts: [{ text: 'hello' }] }],
            });
            expect(resVertex.status).to.equal(503);
        });

        it('Should apply delay_ms and stream_chunk_size / stream_chunk_delay_ms', async () => {
            await client.putMatcher('delayed-stream-rule', {
                priority: 300,
                match: { message_contains: 'slow stream' },
                response: {
                    text: '123456789012345',
                    stream_chunk_size: 5,
                    delay_ms: 40,
                    stream_chunk_delay_ms: 15,
                },
            });

            const start = Date.now();
            const res = await client.createChatCompletion({
                model: 'gpt-4o',
                messages: [{ role: 'user', content: 'run slow stream' }],
                stream: true,
            });
            const raw = await res.text();
            const elapsed = Date.now() - start;
            expect(elapsed).to.be.at.least(50);

            const { chunks } = parseChatSSE(raw);
            const contentChunks = chunks.filter(c => c.choices?.[0]?.delta?.content);
            // 15 chars with chunk size 5 -> 3 content chunks
            expect(contentChunks).to.have.lengthOf(3);
            expect(contentChunks.map(c => c.choices[0].delta.content)).to.deep.equal([
                '12345',
                '67890',
                '12345',
            ]);
        });
    });

    describe('Testing, Inspection & Runtime Management API (/testing/...)', () => {
        it('Should record requests across all APIs and support filtering and clearing', async () => {
            await client.createChatCompletion({
                model: 'gpt-4o',
                messages: [{ role: 'user', content: 'hello chat' }],
            });
            await client.createResponse({
                model: 'gpt-4o-mini',
                input: 'unmatched input for default rule',
            });
            await client.vertexGenerateContent('gemini-1.5-pro', {
                contents: [{ role: 'user', parts: [{ text: 'hello vertex' }] }],
            });

            const all = await client.getRecordedRequests();
            expect(all.count).to.equal(3);
            expect(all.requests).to.have.lengthOf(3);

            // Filter by api_type
            const vertexOnly = await client.getRecordedRequests({ api_type: 'vertex' });
            expect(vertexOnly.count).to.equal(1);
            expect(vertexOnly.requests[0].model).to.equal('gemini-1.5-pro');

            // Filter by model
            const miniOnly = await client.getRecordedRequests({ model: 'gpt-4o-mini' });
            expect(miniOnly.count).to.equal(1);
            expect(miniOnly.requests[0].matched_rule_id).to.equal('__default__');

            // Filter by limit
            const limited = await client.getRecordedRequests({ limit: 2 });
            expect(limited.count).to.equal(2);
            expect(limited.requests[1].api_type).to.equal('vertex');

            // Get single request by ID
            const firstId = all.requests[0].id;
            const singleRes = await client.getRecordedRequestById(firstId);
            expect(singleRes.status).to.equal(200);
            const singleReq = await singleRes.json();
            expect(singleReq.id).to.equal(firstId);
            expect(singleReq.last_user_message).to.equal('hello chat');

            // Unknown request ID -> 404
            const missingRes = await client.getRecordedRequestById('req_nonexistent');
            expect(missingRes.status).to.equal(404);

            // Clear recorded requests
            const clearRes = await client.clearRecordedRequests();
            expect(clearRes).to.have.property('message', 'Recorded requests cleared');

            const afterClear = await client.getRecordedRequests();
            expect(afterClear.count).to.equal(0);
        });

        it('Should support CRUD on matchers via /testing/matchers', async () => {
            const listInitial = await client.listMatchers();
            expect(listInitial.matchers).to.be.an('array');
            expect(listInitial.matchers.length).to.equal(6);

            // Create new matcher -> 201 Created
            const createRes = await client.putMatcher('ping-rule', {
                priority: 150,
                match: { message_contains: 'ping' },
                response: { text: 'pong-v1' },
            });
            expect(createRes.status).to.equal(201);
            const created = await createRes.json();
            expect(created.id).to.equal('ping-rule');

            // Get matcher by ID -> 200 OK
            const getRes = await client.getMatcher('ping-rule');
            expect(getRes.status).to.equal(200);
            expect((await getRes.json()).response.text).to.equal('pong-v1');

            // Update existing matcher -> 200 OK
            const updateRes = await client.putMatcher('ping-rule', {
                priority: 160,
                match: { message_contains: 'ping' },
                response: { text: 'pong-v2' },
            });
            expect(updateRes.status).to.equal(200);
            expect((await updateRes.json()).response.text).to.equal('pong-v2');

            // Invalid regex -> 400 Bad Request
            const badRegexRes = await client.putMatcher('bad-regex', {
                priority: 10,
                match: { message_regex: '[unclosed group' },
                response: { text: 'never' },
            });
            expect(badRegexRes.status).to.equal(400);

            // Delete matcher -> 200 OK
            const delRes = await client.deleteMatcher('ping-rule');
            expect(delRes.status).to.equal(200);
            expect(await delRes.json()).to.have.property('message', 'Matcher deleted');

            // Get deleted matcher -> 404 Not Found
            expect((await client.getMatcher('ping-rule')).status).to.equal(404);
            // Delete non-existent matcher -> 404 Not Found
            expect((await client.deleteMatcher('ping-rule')).status).to.equal(404);
        });

        it('Should support CRUD on models via /testing/models/{id}', async () => {
            // Create new model -> 201 Created
            const createRes = await client.putModel('claude-3-7-sonnet', {
                owned_by: 'anthropic',
                created: 1730000000,
            });
            expect(createRes.status).to.equal(201);
            const created = await createRes.json();
            expect(created).to.deep.equal({
                id: 'claude-3-7-sonnet',
                object: 'model',
                owned_by: 'anthropic',
                created: 1730000000,
            });

            // Update existing model -> 200 OK
            const updateRes = await client.putModel('claude-3-7-sonnet', {
                owned_by: 'anthropic-updated',
                created: 1730000001,
            });
            expect(updateRes.status).to.equal(200);
            expect((await updateRes.json()).owned_by).to.equal('anthropic-updated');

            // Verify via GET /v1/models/{model}
            const modelRes = await client.getModel('claude-3-7-sonnet');
            expect(modelRes.status).to.equal(200);

            // Delete model -> 200 OK
            const delRes = await client.deleteModel('claude-3-7-sonnet');
            expect(delRes.status).to.equal(200);
            expect(await delRes.json()).to.have.property('message', 'Model deleted');

            // Verify 404 on second delete
            expect((await client.deleteModel('claude-3-7-sonnet')).status).to.equal(404);
        });

        it('Should get, patch, and reset runtime configuration via /testing/config and /testing/reset', async () => {
            const initialCfg = await client.getConfig();
            expect(initialCfg.default_response).to.equal('I am a local AI provider default response.');
            expect(initialCfg.features.chat_completions_enabled).to.be.true;

            const patchRes = await client.patchConfig({
                default_response: 'Custom patched fallback',
                delay_ms: 5,
            });
            expect(patchRes.status).to.equal(200);
            const patchedCfg = await patchRes.json();
            expect(patchedCfg.default_response).to.equal('Custom patched fallback');
            expect(patchedCfg.delay_ms).to.equal(5);

            // Reset state
            const resetRes = await client.resetState();
            expect(resetRes).to.have.property('message', 'State reset to initial configuration');

            const afterResetCfg = await client.getConfig();
            expect(afterResetCfg.default_response).to.equal('I am a local AI provider default response.');
            expect(afterResetCfg.delay_ms).to.equal(0);
        });
    });
});
