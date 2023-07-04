from urllib.parse import urlparse
from urllib.robotparser import RobotFileParser

# a dict to store RobotFileParser for each domain
robots_txt_dict = {}

with open('urls.txt', 'r') as file:
    for url in file.readlines():
        url = url.strip()  # remove newline character at the end of the line
        domain = urlparse(url).hostname
        if domain not in robots_txt_dict:
            robots_txt_dict[domain] = RobotFileParser(urlparse(url).scheme + "://" + domain + "/robots.txt")
            robots_txt_dict[domain].read()

        can_fetch = robots_txt_dict[domain].can_fetch("*", url)
        if can_fetch:
            print(url)
