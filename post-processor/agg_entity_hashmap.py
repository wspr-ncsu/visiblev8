#!/usr/bin/env python3

import os
import json

res = {}
for filename in os.listdir('./tracker-radar/entities/'):
    if filename.endswith('.json'):
        with open('./tracker-radar/entities/' + filename) as f:
            data = json.load(f)
            for origin in data['properties']:
                if "prevalence" not in data:
                    res[origin] = { "displayName": data['displayName'], "tracking": 0 }
                else:
                    res[origin] = { "displayName": data['displayName'], "tracking": data["prevalence"]["tracking"] }

with open('./artifacts/entities.json', 'w') as f:
    json.dump(res, f, indent=4)