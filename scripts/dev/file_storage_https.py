"""Render fixed DEV File HTTPS/edge-policy companions; no cluster/host writes."""
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


def network_policy():
    return {
        'apiVersion':'networking.k8s.io/v1','kind':'NetworkPolicy',
        'metadata':{
            'name':'voice-dev-file-storage-edge','namespace':'voice-dev',
            'labels':{'app.kubernetes.io/managed-by':'voice-dev',
                      'voice.environment':'dev',
                      'app.kubernetes.io/component':'file-storage-https'},
        },
        'spec':{
            'podSelector':{'matchLabels':{'app':'voice-minio'}},
            'policyTypes':['Ingress'],
            'ingress':[{
                'from':[{
                    'namespaceSelector':{'matchLabels':{'kubernetes.io/metadata.name':'traefik'}},
                    'podSelector':{'matchLabels':{
                        'app.kubernetes.io/name':'traefik',
                        'app.kubernetes.io/instance':'traefik-traefik',
                    }},
                }],
                'ports':[{'protocol':'TCP','port':9000}],
            }],
        },
    }


def resources():
    return {'apiVersion':'v1','kind':'List','items':[ingress(),network_policy()]}


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action',choices=['render'])
    parser.parse_args()
    print(json.dumps(resources(),indent=2))


if __name__=='__main__':main()
