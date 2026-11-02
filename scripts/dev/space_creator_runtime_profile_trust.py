"""Public certificate-only JDK truststore predicates; no runtime admission."""
import re


def parse(raw):
    if not isinstance(raw,bytes) or not raw or len(raw)>1048576: raise ValueError('trust listing invalid')
    count=re.findall(rb'Your keystore contains (\d+) entr(?:y|ies)',raw)
    kinds=re.findall(rb'Entry type: ([A-Za-z]+)',raw)
    fingerprints=re.findall(rb'SHA256:\s*([0-9A-F]{2}(?::[0-9A-F]{2}){31})',raw)
    if (len(count)!=1 or not 0<int(count[0])<=4096 or len(kinds)!=int(count[0]) or
            len(fingerprints)!=len(kinds) or any(kind!=b'trustedCertEntry' for kind in kinds) or
            b'PrivateKeyEntry' in raw or b'SecretKeyEntry' in raw): raise ValueError('trust listing invalid')
    result={value.replace(b':',b'').decode('ascii').lower() for value in fingerprints}
    return result


def compare_roots(original,store_output,media_sha256):
    if not isinstance(media_sha256,str) or not re.fullmatch(r'[a-f0-9]{64}',media_sha256): raise ValueError('media certificate invalid')
    if not isinstance(original,set) or not original or any(not isinstance(x,str) or not re.fullmatch(r'[a-f0-9]{64}',x) for x in original): raise ValueError('JDK roots invalid')
    if re.findall(rb'^Keystore type: ([^\r\n]+)\r?$',store_output,re.MULTILINE)!=[b'PKCS12']: raise ValueError('trust store type invalid')
    selected=parse(store_output)
    if media_sha256 in original or selected!=original|{media_sha256}: raise ValueError('trust roots differ')
    return {'root_count':len(original),'store_count':len(selected),'media_ca_sha256':media_sha256,
            'scope':'public-truststore-only-no-https-or-issuer-custody'}


def compare(jdk_output,store_output,media_sha256):
    return compare_roots(parse(jdk_output),store_output,media_sha256)
