import { expect } from 'chai';
import {
    AiProviderClient,
    DEFAULT_BASE_URL,
    launchSnapshot,
    teardownSnapshot,
    waitAvailable,
} from './utils/index.mjs';

describe('strict-auth - Authentication, Strict Models & Feature Toggles', () => {
    const client = new AiProviderClient(DEFAULT_BASE_URL);

    before(async () => {
        await launchSnapshot('strict-auth');
        await waitAvailable(DEFAULT_BASE_URL);
    });

    after(async () => {
        await teardownSnapshot('strict-auth');
    });

    beforeEach(async () => {
        await client.resetState();
    });

    describe('Authentication Enforcement (features.require_auth: true)', () => {
        it('Should return 401 Unauthorized when Authorization header is missing on OpenAI endpoints', async () => {
            const resChat = await client.createChatCompletion({
                model: 'gpt-4o',
                messages: [{ role: 'user', content: 'hello' }],
            });
            expect(resChat.status).to.equal(401);

            const resResp = await client.createResponse({
                model: 'gpt-4o',
                input: 'hello',
            });
            expect(resResp.status).to.equal(401);
        });

        it('Should succeed on OpenAI endpoints when Authorization: Bearer <token> is provided', async () => {
            const authHeaders = { Authorization: 'Bearer sk-test-token-123' };

            const resChat = await client.createChatCompletion(
                {
                    model: 'gpt-4o',
                    messages: [{ role: 'user', content: 'hello' }],
                },
                authHeaders
            );
            expect(resChat.status).to.equal(200);
            const bodyChat = await resChat.json();
            expect(bodyChat.choices[0].message.content).to.equal('Authenticated hello response!');

            const resResp = await client.createResponse(
                {
                    model: 'gpt-4o',
                    input: 'hello',
                },
                authHeaders
            );
            expect(resResp.status).to.equal(200);
            const bodyResp = await resResp.json();
            expect(bodyResp.output_text).to.equal('Authenticated hello response!');
        });

        it('Should enforce authentication on Vertex AI endpoints and accept Bearer token, x-goog-api-key header, or ?key= query param', async () => {
            const vertexPayload = {
                contents: [{ role: 'user', parts: [{ text: 'hello' }] }],
            };

            // Missing auth -> 401
            const resUnauth = await client.vertexGenerateContent('gemini-1.5-pro', vertexPayload);
            expect(resUnauth.status).to.equal(401);

            // With Bearer token -> 200
            const resBearer = await client.vertexGenerateContent('gemini-1.5-pro', vertexPayload, {
                Authorization: 'Bearer ya29.test-token',
            });
            expect(resBearer.status).to.equal(200);

            // With x-goog-api-key header -> 200
            const resApiKeyHeader = await client.vertexGenerateContent('gemini-1.5-pro', vertexPayload, {
                'x-goog-api-key': 'AIzaSyTestKey',
            });
            expect(resApiKeyHeader.status).to.equal(200);

            // With ?key= query param -> 200
            const resApiKeyQuery = await client.vertexGenerateContent(
                'gemini-1.5-pro',
                vertexPayload,
                {},
                'key=AIzaSyTestKey'
            );
            expect(resApiKeyQuery.status).to.equal(200);
        });
    });

    describe('Strict Model Validation (features.strict_models: true)', () => {
        const authHeaders = { Authorization: 'Bearer sk-test-token-123' };

        it('Should return 404 Not Found when requesting an unconfigured model on generation endpoints', async () => {
            const resChat = await client.createChatCompletion(
                {
                    model: 'non-existent-gpt',
                    messages: [{ role: 'user', content: 'hello' }],
                },
                authHeaders
            );
            expect(resChat.status).to.equal(404);

            const resResp = await client.createResponse(
                {
                    model: 'non-existent-gpt',
                    input: 'hello',
                },
                authHeaders
            );
            expect(resResp.status).to.equal(404);

            const resVertex = await client.vertexGenerateContent(
                'non-existent-gemini',
                { contents: [{ role: 'user', parts: [{ text: 'hello' }] }] },
                authHeaders
            );
            expect(resVertex.status).to.equal(404);
        });

        it('Should allow generation after dynamically registering the model via PUT /testing/models/{id}', async () => {
            await client.putModel('custom-model-v1', { owned_by: 'custom', created: 1720000000 });

            const resChat = await client.createChatCompletion(
                {
                    model: 'custom-model-v1',
                    messages: [{ role: 'user', content: 'hello' }],
                },
                authHeaders
            );
            expect(resChat.status).to.equal(200);
        });
    });

    describe('Default Responses Cycling & Feature Toggles', () => {
        const authHeaders = { Authorization: 'Bearer sk-test-token-123' };

        it('Should cycle through default_responses when no matcher matches', async () => {
            const r1 = await (
                await client.createChatCompletion(
                    {
                        model: 'gpt-4o',
                        messages: [{ role: 'user', content: 'unmatched 1' }],
                    },
                    authHeaders
                )
            ).json();
            const r2 = await (
                await client.createChatCompletion(
                    {
                        model: 'gpt-4o',
                        messages: [{ role: 'user', content: 'unmatched 2' }],
                    },
                    authHeaders
                )
            ).json();

            expect(r1.choices[0].message.content).to.equal('Strict Fallback 1');
            expect(r2.choices[0].message.content).to.equal('Strict Fallback 2');
        });

        it('Should return 404 when specific API families are disabled via features toggles', async () => {
            // Disable chat_completions_enabled
            await client.patchConfig({
                features: {
                    chat_completions_enabled: false,
                    responses_enabled: true,
                    vertex_enabled: true,
                    strict_models: true,
                    require_auth: true,
                },
            });

            const resChat = await client.createChatCompletion(
                {
                    model: 'gpt-4o',
                    messages: [{ role: 'user', content: 'hello' }],
                },
                authHeaders
            );
            expect(resChat.status).to.equal(404);

            // Responses API still works
            const resResp = await client.createResponse(
                {
                    model: 'gpt-4o',
                    input: 'hello',
                },
                authHeaders
            );
            expect(resResp.status).to.equal(200);

            // Disable responses_enabled and vertex_enabled
            await client.patchConfig({
                features: {
                    chat_completions_enabled: true,
                    responses_enabled: false,
                    vertex_enabled: false,
                    strict_models: true,
                    require_auth: true,
                },
            });

            const resRespDisabled = await client.createResponse(
                {
                    model: 'gpt-4o',
                    input: 'hello',
                },
                authHeaders
            );
            expect(resRespDisabled.status).to.equal(404);

            const resVertexDisabled = await client.vertexGenerateContent(
                'gemini-1.5-pro',
                { contents: [{ role: 'user', parts: [{ text: 'hello' }] }] },
                authHeaders
            );
            expect(resVertexDisabled.status).to.equal(404);
        });
    });
});
