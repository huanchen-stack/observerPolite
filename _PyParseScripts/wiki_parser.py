import bz2
import xml.etree.ElementTree as ET
import mwparserfromhell

NS = {'ns0': 'http://www.mediawiki.org/xml/export-0.10/'}
filename = '/home/huasun/enwiki/all/enwiki-20230601-pages-articles-multistream.xml.bz2'

with bz2.open(filename, 'rt') as f, open('new_all_art_urls_w_title.txt', 'w') as fout:
    title = None
    ns = None
    i = 0
    for event, elem in ET.iterparse(f, events=('start', 'end')):

        tag = elem.tag.split('}', 1)[1]  # strip the namespace
        if event == 'start':
            if tag == 'page':
                title = None
                ns = None
            elif tag == 'title':
                title = elem.text
            elif tag == 'ns':
                ns = elem.text
        else:  # event == 'end'
            if tag == 'text' and ns == '0':
                if elem.text is not None:
                    wikicode = mwparserfromhell.parse(elem.text)
                    external_links = wikicode.filter_external_links()
                    for link in external_links:
                        fout.write(f"{str(link.url)}, {title}\n")

                if (i+1)%1000 == 0:
                    print(f".", end='', flush=True)
                if (i+1)%100000 == 0:
                    print(f" {(i+1)//100000} batch (100k)", flush=True)
                i += 1

            elem.clear()  # discard the element to save memory