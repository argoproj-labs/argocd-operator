#!/bin/sh -e

# Copyright 2020 ArgoCD Operator Developers
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# 	http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

BACKUP_SCRIPT=$0
BACKUP_ACTION=$1
BACKUP_LOCATION=$2
BACKUP_FILENAME=argocd-backup.yaml
BACKUP_EXPORT_LOCATION=/tmp/${BACKUP_FILENAME}
BACKUP_ENCRYPT_LOCATION=/backups/${BACKUP_FILENAME}
BACKUP_KEY_LOCATION=/secrets/backup.key

export_argocd () {
    echo "exporting argo-cd"
    create_backup
    encrypt_backup
    push_backup
    echo "argo-cd export complete"
}

create_backup () {
    echo "creating argo-cd backup"
    argocd-util export > ${BACKUP_EXPORT_LOCATION}
}

encrypt_backup () {
    echo "encrypting argo-cd backup"
    openssl enc -aes-256-cbc -pbkdf2 -pass file:${BACKUP_KEY_LOCATION} -in ${BACKUP_EXPORT_LOCATION} -out ${BACKUP_ENCRYPT_LOCATION}
    rm ${BACKUP_EXPORT_LOCATION}
}

push_backup () {
    case  ${BACKUP_LOCATION} in
        "aws")
            echo "pushing argo-cd backup to aws"
            BACKUP_BUCKET_NAME=`cat /secrets/aws.bucket.name`
            BACKUP_BUCKET_URI="s3://${BACKUP_BUCKET_NAME}"
            aws s3 mb ${BACKUP_BUCKET_URI}
            aws s3api put-public-access-block --bucket ${BACKUP_BUCKET_NAME} --public-access-block-configuration "BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true"
            aws s3 cp ${BACKUP_ENCRYPT_LOCATION} ${BACKUP_BUCKET_URI}/${BACKUP_FILENAME}
            ;;
        "gcp")
            echo "pushing argo-cd backup to gcp"
            BACKUP_BUCKET_KEY="/secrets/gcp.key.file"
            BACKUP_BUCKET_NAME=`cat /secrets/gcp.bucket.name`
            BACKUP_BUCKET_URI="gs://${BACKUP_BUCKET_NAME}"
            gcloud auth activate-service-account --key-file=${BACKUP_BUCKET_KEY}
            gsutil mb -b on ${BACKUP_BUCKET_URI}
            gsutil cp ${BACKUP_ENCRYPT_LOCATION} ${BACKUP_BUCKET_URI}/${BACKUP_FILENAME}
            ;;
        *)
        # local and unsupported backends
    esac
}

import_argocd () {
    echo "importing argo-cd"
    pull_backup
    decrypt_backup
    load_backup
    echo "argo-cd import complete"
}

pull_backup () {
    case  ${BACKUP_LOCATION} in
        "aws")
            echo "pulling argo-cd backup from aws"
            BACKUP_BUCKET_NAME=`cat /secrets/aws.bucket.name`
            BACKUP_BUCKET_URI="s3://${BACKUP_BUCKET_NAME}"
            aws s3 cp ${BACKUP_BUCKET_URI}/${BACKUP_FILENAME} ${BACKUP_ENCRYPT_LOCATION}
            ;;
        "gcp")
            echo "pulling argo-cd backup from gcp"
            BACKUP_BUCKET_KEY="/secrets/gcp.key.file"
            BACKUP_BUCKET_NAME=`cat /secrets/gcp.bucket.name`
            BACKUP_BUCKET_URI="gs://${BACKUP_BUCKET_NAME}"
            gcloud auth activate-service-account --key-file=${BACKUP_BUCKET_KEY}
            gsutil cp ${BACKUP_BUCKET_URI}/${BACKUP_FILENAME} ${BACKUP_ENCRYPT_LOCATION}
            ;;
        *)
        # local and unsupported backends
    esac
}

decrypt_backup () {
    echo "decrypting argo-cd backup"
    openssl enc -aes-256-cbc -d -pbkdf2 -pass file:${BACKUP_KEY_LOCATION} -in ${BACKUP_ENCRYPT_LOCATION} -out ${BACKUP_EXPORT_LOCATION}
}

load_backup () {
    echo "loading argo-cd backup"
    argocd-util import - < ${BACKUP_EXPORT_LOCATION}
}

usage () {
    echo "usage: ${BACKUP_SCRIPT} export|import"
}

case  ${BACKUP_ACTION} in
    "export")
        export_argocd
        ;;
    "import")
        import_argocd
        ;;
    # TODO: Implement finalize action to clean up cloud resources!
    *)
    usage
esac
