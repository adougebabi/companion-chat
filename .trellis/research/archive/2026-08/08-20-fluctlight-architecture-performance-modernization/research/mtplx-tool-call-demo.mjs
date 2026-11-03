import {readFileSync} from 'node:fs';

function loadDotEnv(path = '.env') {
    try {
        const values = {};
        for (const line of readFileSync(path, 'utf8').split(/\r?\n/)) {
            const match = line.match(/^\s*([A-Z][A-Z0-9_]*)\s*=\s*(.*?)\s*$/);
            if (!match) continue;
            values[match[1]] = match[2].replace(/^(['"])(.*)\1$/, '$2');
        }
        return values;
    } catch {
        return {};
    }
}

const env = {...loadDotEnv(), ...process.env};
const baseUrl = String(env.MTPLX_URL || 'http://127.0.0.1:8000/v1').replace(/\/$/, '');
const apiKey = String(env.MTPLX_API_KEY || '').trim();
const model = String(env.MTPLX_MODEL || '').trim();
if (!apiKey || !model) throw new Error('MTPLX_API_KEY and MTPLX_MODEL are required');

const tool = {
    type: 'function',
    function: {
        name: 'get_weather',
        description: 'Return weather for a city.',
        parameters: {
            type: 'object',
            properties: {city: {type: 'string'}},
            required: ['city'],
            additionalProperties: false
        }
    }
};

async function completion(payload) {
    const response = await fetch(`${baseUrl}/chat/completions`, {
        method: 'POST',
        headers: {'Authorization': `Bearer ${apiKey}`, 'Content-Type': 'application/json'},
        body: JSON.stringify(payload)
    });
    const text = await response.text();
    if (!response.ok) throw new Error(`HTTP ${response.status}: ${text.slice(0, 500)}`);
    return JSON.parse(text);
}

function summarizeToolCall(call) {
    return {
        id: call?.id || null,
        type: call?.type || null,
        name: call?.function?.name || null,
        arguments: call?.function?.arguments || ''
    };
}

async function streamCompletion(payload) {
    const response = await fetch(`${baseUrl}/chat/completions`, {
        method: 'POST',
        headers: {'Authorization': `Bearer ${apiKey}`, 'Content-Type': 'application/json'},
        body: JSON.stringify({...payload, stream: true})
    });
    if (!response.ok) throw new Error(`HTTP ${response.status}: ${(await response.text()).slice(0, 500)}`);
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    let finishReason = null;
    const calls = [];
    const chunks = [];
    const consume = raw => {
        buffer += raw;
        const frames = buffer.split(/\r?\n\r?\n/);
        buffer = frames.pop() || '';
        for (const frame of frames) {
            const data = frame.split(/\r?\n/).find(line => line.startsWith('data:'))?.slice(5).trim();
            if (!data || data === '[DONE]') continue;
            const chunk = JSON.parse(data);
            chunks.push(chunk);
            const choice = chunk.choices?.[0];
            finishReason ||= choice?.finish_reason || null;
            for (const fragment of choice?.delta?.tool_calls || []) {
                const index = Number.isInteger(fragment.index) ? fragment.index : calls.length;
                const call = calls[index] ||= {id: '', type: 'function', function: {name: '', arguments: ''}};
                call.id ||= fragment.id || '';
                call.type ||= fragment.type || 'function';
                call.function.name += fragment.function?.name || '';
                call.function.arguments += fragment.function?.arguments || '';
            }
        }
    };
    while (true) {
        const {done, value} = await reader.read();
        if (done) break;
        consume(decoder.decode(value, {stream: true}));
    }
    consume(decoder.decode());
    return {chunks, finishReason, calls: calls.map(summarizeToolCall)};
}

const basePayload = {
    model,
    messages: [{role: 'user', content: 'Call the weather tool for Shanghai. Do not answer directly.'}],
    tools: [tool],
    tool_choice: 'required',
    max_tokens: 64
};

const nonStreaming = await completion({...basePayload, stream: false});
const nonStreamingChoice = nonStreaming.choices?.[0];
console.log(JSON.stringify({
    test: 'non_streaming_tool_call',
    model: nonStreaming.model,
    finishReason: nonStreamingChoice?.finish_reason || null,
    toolCalls: (nonStreamingChoice?.message?.tool_calls || []).map(summarizeToolCall)
}, null, 2));

const streaming = await streamCompletion(basePayload);
console.log(JSON.stringify({
    test: 'streaming_tool_call',
    chunkCount: streaming.chunks.length,
    finishReason: streaming.finishReason,
    toolCalls: streaming.calls
}, null, 2));

const firstCall = nonStreamingChoice?.message?.tool_calls?.[0];
if (firstCall) {
    const followUp = await completion({
        model,
        messages: [
            ...basePayload.messages,
            {role: 'assistant', content: null, tool_calls: [firstCall]},
            {role: 'tool', tool_call_id: firstCall.id, name: firstCall.function.name, content: '{"city":"Shanghai","temperatureC":26,"condition":"sunny"}'}
        ],
        tools: [tool],
        tool_choice: 'none',
        stream: false,
        max_tokens: 256
    });
    const followUpChoice = followUp.choices?.[0];
    console.log(JSON.stringify({
        test: 'tool_result_follow_up',
        finishReason: followUpChoice?.finish_reason || null,
        content: followUpChoice?.message?.content || ''
    }, null, 2));
}
