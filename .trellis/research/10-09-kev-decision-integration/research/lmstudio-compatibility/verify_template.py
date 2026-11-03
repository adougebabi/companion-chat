#!/usr/bin/env python3
"""Offline candidate-template verification; no inference or service changes."""
import json
from pathlib import Path
import unittest
from jinja2 import Environment, StrictUndefined

ROOT = Path(__file__).parent
ORIGINAL = (ROOT/'original-template.jinja').read_text()
CANDIDATE = (ROOT/'native-tools-v7.jinja').read_text()


def reject(message):
    raise ValueError(message)


def render(template, messages, tools=None):
    environment = Environment(undefined=StrictUndefined)
    environment.globals.update(strftime_now=lambda _: '2026-10-13', raise_exception=reject)
    values = {'messages': messages, 'bos_token': '<s>', 'eos_token': '</s>'}
    if tools is not None:
        values['tools'] = tools
    return environment.from_string(template).render(**values).strip()


def definition(name='probe.read'):
    return {'type':'function','function':{'name':name,'description':'Synthetic read-only probe',
        'parameters':{'type':'object','properties':{'value':{'type':'string'}},'required':['value'], 'additionalProperties':False}}}


class TemplateVerification(unittest.TestCase):
    def test_no_tools_preserves_text_and_non_alternating_roles(self):
        messages=[{'role':'system','content':'trusted instructions'},
            {'role':'user','content':'[RUNTIME CONTEXT] scoped data [/RUNTIME CONTEXT]'},
            {'role':'user','content':'historical user'}, {'role':'assistant','content':'history'},
            {'role':'user','content':'current request'}, {'role':'user','content':'correction'}]
        self.assertEqual(render(ORIGINAL,messages),render(CANDIDATE,messages))

    def test_tools_are_delivered_and_changing_name_changes_input(self):
        messages=[{'role':'user','content':'invoke the provided function'}]
        before=render(ORIGINAL,messages,[definition()])
        self.assertEqual(before,render(ORIGINAL,messages,[definition('other_probe')]))
        after=render(CANDIDATE,messages,[definition()])
        self.assertIn('[AVAILABLE_TOOLS]',after)
        self.assertIn('probe.read',after)
        self.assertNotEqual(after,render(CANDIDATE,messages,[definition('other_probe')]))

    def test_native_call_arguments_and_ids_are_preserved(self):
        for args in [{'value':'中文'}]:
            messages=[{'role':'user','content':'call'}, {'role':'assistant','content':None,
                'tool_calls':[{'id':'probe0001','type':'function','function':{'name':'probe.read','arguments':args}}]}]
            value=render(CANDIDATE,messages,[definition()])
            calls=json.loads(value.split('[TOOL_CALLS]',1)[1].removesuffix('</s>'))
            self.assertEqual(calls,[{'name':'probe.read','arguments':{'value':'中文'},'id':'probe0001'}])

    def test_repeated_user_messages_emit_one_catalog(self):
        messages=[{'role':'user','content':'same'},{'role':'assistant','content':'history'},
            {'role':'user','content':'same'}]
        value=render(CANDIDATE,messages,[definition()])
        self.assertEqual(value.count('[AVAILABLE_TOOLS]'),1)

    def test_malformed_argument_strings_are_escaped_as_data(self):
        # Official V7 parses valid API argument JSON before template rendering.
        # Malformed strings remain strings; this verifies the template fallback,
        # not that LM Studio performs that normalization (live round-trip pending).
        for args in ['{','']:
            messages=[{'role':'user','content':'call'}, {'role':'assistant','content':None,
                'tool_calls':[{'id':'probe0001','type':'function','function':{'name':'probe.read','arguments':args}}]}]
            value=render(CANDIDATE,messages,[definition()])
            calls=json.loads(value.split('[TOOL_CALLS]',1)[1].removesuffix('</s>'))
            self.assertEqual(calls[0]['arguments'],args)

    def test_multiple_calls_are_not_dropped(self):
        calls=[{'id':id,'type':'function','function':{'name':name,'arguments':{}}}
            for id,name in [('probe0001','probe.read'),('probe0002','probe.other')]]
        messages=[{'role':'user','content':'call'},{'role':'assistant','content':'preface','tool_calls':calls}]
        value=render(CANDIDATE,messages,[definition()])
        self.assertIn('preface[TOOL_CALLS]',value)
        self.assertEqual([x['id'] for x in json.loads(value.split('[TOOL_CALLS]',1)[1].removesuffix('</s>'))],['probe0001','probe0002'])

    def test_tool_result_is_supported_and_preserves_pairing(self):
        messages=[{'role':'user','content':'call'}, {'role':'tool','tool_call_id':'probe0001','content':'PROBE_RESULT_7318'}]
        with self.assertRaises(ValueError):render(ORIGINAL,messages,[definition()])
        self.assertIn('[TOOL_RESULTS]probe0001[TOOL_CONTENT]PROBE_RESULT_7318[/TOOL_RESULTS]',render(CANDIDATE,messages,[definition()]))

    def test_uses_v7_tool_tokens_not_v11(self):
        for token in ['[AVAILABLE_TOOLS]','[/AVAILABLE_TOOLS]','[TOOL_CALLS]', '[TOOL_RESULTS]','[TOOL_CONTENT]','[/TOOL_RESULTS]']:
            self.assertIn(token,CANDIDATE)
        self.assertNotIn('[CALL_ID]',CANDIDATE)
        self.assertNotIn('[ARGS]',CANDIDATE)


if __name__=='__main__':unittest.main(verbosity=2)
