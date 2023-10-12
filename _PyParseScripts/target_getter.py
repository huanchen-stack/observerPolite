import json
from urllib.parse import urlparse

with open("new_all_domain_count.json", 'r') as f:
    d = json.load(f)

with open("new_all_art_urls_w_title.txt", "r") as f:
    with open("new_target_urls.txt", "w") as fout:
        i = 0
        j = 0
        for line in f:
            url = line.split(',')[0].strip()
            o = urlparse(url)
            if o.hostname is None:
                continue
            elif d[o.hostname] <= 1000:
                fout.write(f"{line}")
                j += 1
            if (i+1)%100000 == 0:
                print(".", end='', flush=True)
            if (i+1)%5000000 == 0:
                print("", flush=True)
            i += 1
        print(j)