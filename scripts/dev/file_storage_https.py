"""Render the fixed DEV HTTPS File bucket companion; no cluster/host writes."""
import argparse
import json


def ingress():
    def path(value,kind):
        return {'path':value,'pathType':kind,
                'backend':{'service':{'name':'voice-minio','port':{'number':9000}}}}
    return {
        'apiVersion':'networking.k8s.io/v1','kind':'Ingress',
        'metadata':{
            'name':'voice-dev-file-storage-https','namespace':'voice-dev',
            'labels':{'app.kubernetes.io/managed-by':'voice-dev',
                      'voice.environment':'dev',
                      'app.kubernetes.io/component':'file-storage-https'},
            'annotations':{'traefik.ingress.kubernetes.io/router.entrypoints':'websecure'},
        },
        'spec':{
            'ingressClassName':'traefik',
            'tls':[{'hosts':['api-dev.comrade.click'],
                    'secretName':'voice-dev-signaling-tls'}],
            'rules':[{'host':'api-dev.comrade.click','http':{'paths':[
                path('/voice-dev-files','Exact'),
                path('/voice-dev-files/','Prefix'),
            ]}}],
        },
    }


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action',choices=['render'])
    parser.parse_args()
    print(json.dumps(ingress(),indent=2))


if __name__=='__main__':main()
