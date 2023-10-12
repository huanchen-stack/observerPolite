from urllib.parse import urlparse
import json


d = {}

with open("new_all_art_urls_w_title.txt", 'r') as f:
    i = 0
    for line in f:
        url = line.split(',')[0].strip()
        o = urlparse(url)
        if o.hostname is None:
            continue
        elif o.hostname not in d:
            d[o.hostname] = 0
        d[o.hostname] += 1
        if (i+1)%100000 == 0:
            print(".", end='', flush=True)
        if (i+1)%5000000 == 0:
            print("", flush=True)
        i += 1

with open("new_all_domain_count.json", 'w') as f:
    json.dump(d, f)