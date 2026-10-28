import { expect } from 'chai';
import {
    AiProviderClient,
    DEFAULT_BASE_URL,
    launchSnapshot,
    teardownSnapshot,
    waitAvailable,
    parseChatSSE,
    parseResponsesSSE,
    parseVertexSSE,
} from './utils/index.mjs';

describe('simple - Core AI Provider APIs', () => {
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

    describe('Health Check & CORS', () => {
        it('GET /healthz should return status OK', async () => {
            const health = await client.healthz();
            expect(health).to.deep.equal({ status: 'OK' });
        });

        it('OPTIONS preflight should return 204 with CORS headers', async () => {
            const res = await fetch(`${DEFAULT_BASE_URL}/v1/chat/completions`, {
                method: 'OPTIONS',
                headers: {
                    'Origin': 'http://localhost:3000',
                    'Access-Control-Request-Method': 'POST',
                    'Access-Control-Request-Headers': 'Content-Type, Authorization',
                },
            });
            expect(res.status).to.equal(204);
            expect(res.headers.get('Access-Control-Allow-Origin')).to.exist;
        });
    });

    describe('OpenAI Models API', () => {
        it('GET /v1/models should list all configured models', async () => {
            const res = await client.listModels();
            expect(res).to.have.property('object', 'list');
            expect(res.data).to.be.an('array').with.lengthOf(3);

            const gpt4o = res.data.find(m => m.id === 'gpt-4o');
            expect(gpt4o).to.exist;
            expect(gpt4o).to.have.property('object', 'model');
            expect(gpt4o).to.have.property('owned_by', 'openai');
            expect(gpt4o).to.have.property('created', 1715367049);

            const gemini = res.data.find(m => m.id === 'gemini-1.5-pro');
            expect(gemini).to.exist;
            expect(gemini).to.have.property('owned_by', 'google');
        });

        it('GET /v1/models/{model} should return a specific model by ID', async () => {
            const res = await client.getModel('gpt-4o');
            expect(res.status).to.equal(200);
            const body = await res.json();
            expect(body).to.deep.equal({
                id: 'gpt-4o',
                object: 'model',
                created: 1715367049,
                owned_by: 'openai',
            });
        });

        it('GET /v1/models/{model} should return 404 model_not_found for unknown model', async () => {
            const res = await client.getModel('unknown-model');
            expect(res.status).to.equal(404);
            const body = await res.json();
            expect(body).to.have.property('error');
            expect(body.error).to.include({
                message: "The model 'unknown-model' does not exist",
                type: 'invalid_request_error',
                param: 'model',
                code: 'model_not_found',
            });
        });
    });

    describe('OpenAI Chat Completions API (POST /v1/chat/completions)', () => {
        it('Should return synchronous text completion matching regex rule', async () => {
            const res = await client.createChatCompletion({
                model: 'gpt-4o',
                messages: [
                    { role: 'system', content: 'You are a helpful assistant.' },
                    { role: 'user', content: 'Hello there!' },
                ],
                stream: false,
            });
            expect(res.status).to.equal(200);
            const body = await res.json();
            expect(body).to.have.property('id').that.is.a('string');
            expect(body).to.have.property('object', 'chat.completion');
            expect(body).to.have.property('created').that.is.a('number');
            expect(body).to.have.property('model', 'gpt-4o');
            expect(body).to.have.property('system_fingerprint', 'fp_local_ai_provider');
            expect(body.choices).to.be.an('array').with.lengthOf(1);
            expect(body.choices[0]).to.have.property('index', 0);
            expect(body.choices[0]).to.have.property('finish_reason', 'stop');
            expect(body.choices[0].message).to.include({
                role: 'assistant',
                content: 'Hello! How can I assist you today?',
                refusal: null,
            });
            expect(body.usage).to.have.property('prompt_tokens').that.is.a('number');
            expect(body.usage).to.have.property('completion_tokens').that.is.a('number');
            expect(body.usage).to.have.property('total_tokens').that.is.a('number');
            expect(body.usage).to.have.property('prompt_tokens_details');
            expect(body.usage).to.have.property('completion_tokens_details');
        });

        it('Should support array of content parts in user message', async () => {
            const res = await client.createChatCompletion({
                model: 'gpt-4o',
                messages: [
                    {
                        role: 'user',
                        content: [
                            { type: 'text', text: 'Hello ' },
                            { type: 'text', text: 'from content parts!' },
                        ],
                    },
                ],
            });
            expect(res.status).to.equal(200);
            const body = await res.json();
            expect(body.choices[0].message.content).to.equal('Hello! How can I assist you today?');
        });

        it('Should include reasoning_content when configured on matched rule', async () => {
            const res = await client.createChatCompletion({
                model: 'gpt-4o',
                messages: [
                    { role: 'user', content: 'Please solve step by step: what is 6 * 7?' },
                ],
            });
            expect(res.status).to.equal(200);
            const body = await res.json();
            expect(body.choices[0].message.content).to.equal('The final answer is 42.');
            expect(body.choices[0].message.reasoning_content).to.equal('Step 1: Analyze the problem. Step 2: Compute result.');
        });

        it('Should return tool_calls and support multi-turn tool role output matching', async () => {
            const tools = [
                {
                    type: 'function',
                    function: {
                        name: 'get_weather',
                        description: 'Get weather',
                        parameters: {
                            type: 'object',
                            properties: { location: { type: 'string' } },
                            required: ['location'],
                        },
                    },
                },
            ];

            // Turn 1: Trigger tool call
            const res1 = await client.createChatCompletion({
                model: 'gpt-4o',
                messages: [
                    { role: 'user', content: 'What is the weather in San Francisco?' },
                ],
                tools,
                tool_choice: 'auto',
            });
            expect(res1.status).to.equal(200);
            const body1 = await res1.json();
            expect(body1.choices[0].finish_reason).to.equal('tool_calls');
            expect(body1.choices[0].message.content).to.be.null;
            expect(body1.choices[0].message.tool_calls).to.be.an('array').with.lengthOf(1);
            const toolCall = body1.choices[0].message.tool_calls[0];
            expect(toolCall).to.deep.equal({
                id: 'call_weather_1',
                type: 'function',
                function: {
                    name: 'get_weather',
                    arguments: '{"location":"San Francisco, CA","unit":"celsius"}',
                },
            });

            // Turn 2: Provide tool output in conversation history
            const res2 = await client.createChatCompletion({
                model: 'gpt-4o',
                messages: [
                    { role: 'user', content: 'What is the weather in San Francisco?' },
                    {
                        role: 'assistant',
                        content: null,
                        tool_calls: [toolCall],
                    },
                    {
                        role: 'tool',
                        tool_call_id: 'call_weather_1',
                        content: '{"temperature": 18, "condition": "sunny"}',
                    },
                ],
                tools,
            });
            expect(res2.status).to.equal(200);
            const body2 = await res2.json();
            expect(body2.choices[0].finish_reason).to.equal('stop');
            expect(body2.choices[0].message.content).to.equal('The current weather in San Francisco is 18°C and sunny.');
        });

        it('Should stream SSE chunks when stream: true', async () => {
            const res = await client.createChatCompletion({
                model: 'gpt-4o',
                messages: [{ role: 'user', content: 'Hello streaming!' }],
                stream: true,
            });
            expect(res.status).to.equal(200);
            expect(res.headers.get('content-type')).to.include('text/event-stream');

            const raw = await res.text();
            const { chunks, done } = parseChatSSE(raw);
            expect(done).to.be.true;
            expect(chunks.length).to.be.at.least(3);

            for (const chunk of chunks) {
                expect(chunk).to.have.property('object', 'chat.completion.chunk');
                expect(chunk).to.have.property('model', 'gpt-4o');
                expect(chunk).to.have.property('system_fingerprint', 'fp_local_ai_provider');
            }

            // First chunk initializes role
            expect(chunks[0].choices[0].delta).to.have.property('role', 'assistant');

            // Reassemble content
            const combined = chunks
                .map(c => c.choices?.[0]?.delta?.content || '')
                .join('');
            expect(combined).to.equal('Hello! How can I assist you today?');

            // Last chunk has finish_reason: "stop"
            const lastChoiceChunk = chunks[chunks.length - 1];
            expect(lastChoiceChunk.choices[0].finish_reason).to.equal('stop');
        });

        it('Should emit usage chunk before [DONE] when stream_options.include_usage is true', async () => {
            const res = await client.createChatCompletion({
                model: 'gpt-4o',
                messages: [{ role: 'user', content: 'Hello with usage!' }],
                stream: true,
                stream_options: { include_usage: true },
            });
            expect(res.status).to.equal(200);
            const raw = await res.text();
            const { chunks, done } = parseChatSSE(raw);
            expect(done).to.be.true;

            const usageChunk = chunks[chunks.length - 1];
            expect(usageChunk.choices).to.be.an('array').that.is.empty;
            expect(usageChunk.usage).to.be.an('object');
            expect(usageChunk.usage).to.have.property('prompt_tokens');
            expect(usageChunk.usage).to.have.property('completion_tokens');
            expect(usageChunk.usage).to.have.property('total_tokens');
        });

        it('Should stream tool call deltas when stream: true and rule returns tool_calls', async () => {
            const res = await client.createChatCompletion({
                model: 'gpt-4o',
                messages: [{ role: 'user', content: 'What is the weather in San Francisco?' }],
                tools: [
                    {
                        type: 'function',
                        function: { name: 'get_weather', parameters: { type: 'object' } },
                    },
                ],
                stream: true,
            });
            expect(res.status).to.equal(200);
            const raw = await res.text();
            const { chunks, done } = parseChatSSE(raw);
            expect(done).to.be.true;

            const toolCallChunks = chunks.filter(c => c.choices?.[0]?.delta?.tool_calls);
            expect(toolCallChunks.length).to.be.at.least(1);
            const tcDelta = toolCallChunks[0].choices[0].delta.tool_calls[0];
            expect(tcDelta).to.have.property('index', 0);
            expect(tcDelta).to.have.property('id', 'call_weather_1');
            expect(tcDelta.function).to.have.property('name', 'get_weather');
            expect(tcDelta.function).to.have.property('arguments', '{"location":"San Francisco, CA","unit":"celsius"}');

            const lastChunk = chunks[chunks.length - 1];
            expect(lastChunk.choices[0].finish_reason).to.equal('tool_calls');
        });

        it('Should return 400 Bad Request when model or messages are missing or JSON is malformed', async () => {
            const resNoModel = await client.createChatCompletion({
                messages: [{ role: 'user', content: 'hi' }],
            });
            expect(resNoModel.status).to.equal(400);

            const resNoMessages = await client.createChatCompletion({
                model: 'gpt-4o',
                messages: [],
            });
            expect(resNoMessages.status).to.equal(400);

            const resBadJson = await client.createChatCompletion('{invalid json');
            expect(resBadJson.status).to.equal(400);
        });
    });

    describe('OpenAI Responses API (POST /v1/responses)', () => {
        it('Should return synchronous text output for string input and instructions', async () => {
            const res = await client.createResponse({
                model: 'gpt-4o',
                instructions: 'You are a helpful assistant.',
                input: 'Hello from Responses API!',
                stream: false,
            });
            expect(res.status).to.equal(200);
            const body = await res.json();
            expect(body).to.have.property('id').that.is.a('string');
            expect(body).to.have.property('object', 'response');
            expect(body).to.have.property('status', 'completed');
            expect(body).to.have.property('model', 'gpt-4o');
            expect(body).to.have.property('instructions', 'You are a helpful assistant.');
            expect(body).to.have.property('output_text', 'Hello! How can I assist you today?');
            expect(body.output).to.be.an('array').with.lengthOf(1);
            expect(body.output[0]).to.include({
                type: 'message',
                status: 'completed',
                role: 'assistant',
            });
            expect(body.output[0].content).to.be.an('array').with.lengthOf(1);
            expect(body.output[0].content[0]).to.deep.equal({
                type: 'output_text',
                text: 'Hello! How can I assist you today?',
                annotations: [],
            });
            expect(body.usage).to.have.property('input_tokens');
            expect(body.usage).to.have.property('output_tokens');
            expect(body.usage).to.have.property('total_tokens');
        });

        it('Should support structured array input and function calling lifecycle', async () => {
            const tools = [
                {
                    type: 'function',
                    name: 'get_weather',
                    description: 'Get the current weather in a given location',
                    parameters: {
                        type: 'object',
                        properties: { location: { type: 'string' } },
                        required: ['location'],
                    },
                    strict: false,
                },
            ];

            // Turn 1: Trigger function_call output item
            const res1 = await client.createResponse({
                model: 'gpt-4o',
                instructions: 'You are a weather assistant.',
                input: [
                    {
                        type: 'message',
                        role: 'user',
                        content: [{ type: 'input_text', text: 'What is the weather in San Francisco?' }],
                    },
                ],
                tools,
            });
            expect(res1.status).to.equal(200);
            const body1 = await res1.json();
            expect(body1.status).to.equal('completed');
            expect(body1.output).to.be.an('array').with.lengthOf(1);
            expect(body1.output[0]).to.include({
                type: 'function_call',
                call_id: 'call_weather_1',
                name: 'get_weather',
                arguments: '{"location":"San Francisco, CA","unit":"celsius"}',
                status: 'completed',
            });

            // Turn 2: Multi-turn array input with function_call and function_call_output
            const res2 = await client.createResponse({
                model: 'gpt-4o',
                instructions: 'You are a weather assistant.',
                input: [
                    {
                        type: 'message',
                        role: 'user',
                        content: [{ type: 'input_text', text: 'What is the weather in San Francisco?' }],
                    },
                    {
                        type: 'function_call',
                        id: body1.output[0].id,
                        call_id: 'call_weather_1',
                        name: 'get_weather',
                        arguments: '{"location":"San Francisco, CA","unit":"celsius"}',
                    },
                    {
                        type: 'function_call_output',
                        call_id: 'call_weather_1',
                        output: '{"temperature": 18, "condition": "sunny"}',
                    },
                ],
                tools,
            });
            expect(res2.status).to.equal(200);
            const body2 = await res2.json();
            expect(body2.output_text).to.equal('The current weather in San Francisco is 18°C and sunny.');
            expect(body2.output[0].type).to.equal('message');
            expect(body2.output[0].content[0].text).to.equal('The current weather in San Francisco is 18°C and sunny.');
        });

        it('Should stream semantic SSE events for text responses when stream: true', async () => {
            const res = await client.createResponse({
                model: 'gpt-4o',
                input: 'Hello semantic streaming!',
                stream: true,
            });
            expect(res.status).to.equal(200);
            expect(res.headers.get('content-type')).to.include('text/event-stream');

            const raw = await res.text();
            const { events, done } = parseResponsesSSE(raw);
            expect(done).to.be.true;

            const eventNames = events.map(e => e.event);
            expect(eventNames[0]).to.equal('response.created');
            expect(eventNames[1]).to.equal('response.in_progress');
            expect(eventNames).to.include('response.output_item.added');
            expect(eventNames).to.include('response.content_part.added');
            expect(eventNames).to.include('response.output_text.delta');
            expect(eventNames).to.include('response.output_text.done');
            expect(eventNames).to.include('response.content_part.done');
            expect(eventNames).to.include('response.output_item.done');
            expect(eventNames[eventNames.length - 1]).to.equal('response.completed');

            const deltas = events
                .filter(e => e.event === 'response.output_text.delta')
                .map(e => e.data.delta)
                .join('');
            expect(deltas).to.equal('Hello! How can I assist you today?');

            const completedEvent = events.find(e => e.event === 'response.completed');
            expect(completedEvent.data.response.status).to.equal('completed');
            expect(completedEvent.data.response.output_text).to.equal('Hello! How can I assist you today?');
        });

        it('Should stream semantic SSE events for function_call responses when stream: true', async () => {
            const res = await client.createResponse({
                model: 'gpt-4o',
                input: 'What is the weather in San Francisco?',
                tools: [{ type: 'function', name: 'get_weather', parameters: { type: 'object' } }],
                stream: true,
            });
            expect(res.status).to.equal(200);
            const raw = await res.text();
            const { events, done } = parseResponsesSSE(raw);
            expect(done).to.be.true;

            const eventNames = events.map(e => e.event);
            expect(eventNames).to.include('response.created');
            expect(eventNames).to.include('response.in_progress');
            expect(eventNames).to.include('response.output_item.added');
            expect(eventNames).to.include('response.function_call_arguments.delta');
            expect(eventNames).to.include('response.function_call_arguments.done');
            expect(eventNames).to.include('response.output_item.done');
            expect(eventNames[eventNames.length - 1]).to.equal('response.completed');

            const argsDone = events.find(e => e.event === 'response.function_call_arguments.done');
            expect(argsDone.data).to.include({
                call_id: 'call_weather_1',
                name: 'get_weather',
                arguments: '{"location":"San Francisco, CA","unit":"celsius"}',
            });
        });

        it('Should return 400 Bad Request when model or input is missing or JSON is malformed', async () => {
            const resNoModel = await client.createResponse({ input: 'Hello' });
            expect(resNoModel.status).to.equal(400);

            const resNoInput = await client.createResponse({ model: 'gpt-4o' });
            expect(resNoInput.status).to.equal(400);

            const resBadJson = await client.createResponse('{bad json');
            expect(resBadJson.status).to.equal(400);
        });
    });

    describe('Vertex AI / Gemini GenerateContent API', () => {
        it('Should generate synchronous text response on /v1beta/models/{model}:generateContent', async () => {
            const res = await client.vertexGenerateContent('gemini-1.5-pro', {
                systemInstruction: {
                    role: 'system',
                    parts: [{ text: 'You are a helpful assistant.' }],
                },
                contents: [
                    {
                        role: 'user',
                        parts: [{ text: 'Hello Gemini!' }],
                    },
                ],
            });
            expect(res.status).to.equal(200);
            const body = await res.json();
            expect(body.modelVersion).to.equal('gemini-1.5-pro');
            expect(body.candidates).to.be.an('array').with.lengthOf(1);
            expect(body.candidates[0]).to.have.property('finishReason', 'STOP');
            expect(body.candidates[0]).to.have.property('index', 0);
            expect(body.candidates[0].content).to.deep.equal({
                role: 'model',
                parts: [{ text: 'Hello! How can I assist you today?' }],
            });
            expect(body.usageMetadata).to.include.keys('promptTokenCount', 'candidatesTokenCount', 'totalTokenCount');
        });

        it('Should generate synchronous text response on /v1/projects/{project}/locations/{location}/publishers/google/models/{model}:generateContent', async () => {
            const res = await client.vertexProjectGenerateContent('my-gcp-project', 'us-central1', 'gemini-1.5-pro', {
                contents: [
                    {
                        role: 'user',
                        parts: [{ text: 'Hello from Vertex project endpoint!' }],
                    },
                ],
            });
            expect(res.status).to.equal(200);
            const body = await res.json();
            expect(body.modelVersion).to.equal('gemini-1.5-pro');
            expect(body.candidates[0].content.parts[0].text).to.equal('Hello! How can I assist you today?');
        });

        it('Should support Vertex functionCall and multi-turn functionResponse', async () => {
            const tools = [
                {
                    functionDeclarations: [
                        {
                            name: 'get_weather',
                            description: 'Get the current weather for a location',
                            parameters: {
                                type: 'OBJECT',
                                properties: { location: { type: 'STRING' } },
                                required: ['location'],
                            },
                        },
                    ],
                },
            ];

            // Turn 1: Trigger functionCall
            const res1 = await client.vertexGenerateContent('gemini-1.5-pro', {
                contents: [
                    {
                        role: 'user',
                        parts: [{ text: 'What is the weather in San Francisco?' }],
                    },
                ],
                tools,
            });
            expect(res1.status).to.equal(200);
            const body1 = await res1.json();
            expect(body1.candidates[0].finishReason).to.equal('STOP');
            const part = body1.candidates[0].content.parts[0];
            expect(part).to.have.property('functionCall');
            expect(part.functionCall.name).to.equal('get_weather');
            expect(part.functionCall.args).to.deep.equal({
                location: 'San Francisco, CA',
                unit: 'celsius',
            });

            // Turn 2: Send functionResponse back
            const res2 = await client.vertexProjectGenerateContent('test-proj', 'europe-west1', 'gemini-1.5-pro', {
                contents: [
                    {
                        role: 'user',
                        parts: [{ text: 'What is the weather in San Francisco?' }],
                    },
                    {
                        role: 'model',
                        parts: [{ functionCall: part.functionCall }],
                    },
                    {
                        role: 'user',
                        parts: [
                            {
                                functionResponse: {
                                    name: 'get_weather',
                                    response: { temperature: 18, condition: 'sunny' },
                                },
                            },
                        ],
                    },
                ],
                tools,
            });
            expect(res2.status).to.equal(200);
            const body2 = await res2.json();
            expect(body2.candidates[0].content.parts[0].text).to.equal(
                'The current weather in San Francisco is 18°C and sunny.'
            );
        });

        it('Should stream SSE chunks on both /v1beta and /v1/projects :streamGenerateContent endpoints', async () => {
            const res1 = await client.vertexStreamGenerateContent('gemini-1.5-pro', {
                contents: [{ role: 'user', parts: [{ text: 'Hello Vertex stream!' }] }],
            });
            expect(res1.status).to.equal(200);
            expect(res1.headers.get('content-type')).to.include('text/event-stream');
            const { chunks: chunks1 } = parseVertexSSE(await res1.text());
            expect(chunks1.length).to.be.at.least(1);
            const text1 = chunks1.map(c => c.candidates?.[0]?.content?.parts?.[0]?.text || '').join('');
            expect(text1).to.equal('Hello! How can I assist you today?');
            expect(chunks1[chunks1.length - 1].candidates[0].finishReason).to.equal('STOP');

            // Reset to get first sequential greeting again
            await client.resetState();

            const res2 = await client.vertexProjectStreamGenerateContent(
                'my-proj',
                'us-central1',
                'gemini-1.5-pro',
                {
                    contents: [{ role: 'user', parts: [{ text: 'Hello Vertex project stream!' }] }],
                },
                {},
                'alt=sse'
            );
            expect(res2.status).to.equal(200);
            const { chunks: chunks2 } = parseVertexSSE(await res2.text());
            const text2 = chunks2.map(c => c.candidates?.[0]?.content?.parts?.[0]?.text || '').join('');
            expect(text2).to.equal('Hello! How can I assist you today?');
        });

        it('Should return JSON array when ?alt=json is passed to :streamGenerateContent', async () => {
            const res = await client.vertexStreamGenerateContent(
                'gemini-1.5-pro',
                {
                    contents: [{ role: 'user', parts: [{ text: 'Hello JSON stream!' }] }],
                },
                {},
                'alt=json'
            );
            expect(res.status).to.equal(200);
            const arr = await res.json();
            expect(arr).to.be.an('array');
            expect(arr.length).to.be.at.least(1);
            const fullText = arr.map(c => c.candidates?.[0]?.content?.parts?.[0]?.text || '').join('');
            expect(fullText).to.equal('Hello! How can I assist you today?');
        });

        it('Should return 400 Bad Request when contents is empty/missing or JSON is malformed', async () => {
            const resEmpty = await client.vertexGenerateContent('gemini-1.5-pro', { contents: [] });
            expect(resEmpty.status).to.equal(400);

            const resMissing = await client.vertexGenerateContent('gemini-1.5-pro', {});
            expect(resMissing.status).to.equal(400);

            const resMalformed = await client.vertexGenerateContent('gemini-1.5-pro', '{malformed');
            expect(resMalformed.status).to.equal(400);
        });
    });
});
