#!/bin/bash

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

# BACKUP_EXPORT_FILE=/tmp/argocd-backup.yaml
# BACKUP_ENCRYPT_FILE=/backups/argocd-backup.yaml
# BACKUP_KEY_FILE=/etc/argocd/backup.key
BACKUP_FILENAME=argocd-backup.yaml
BACKUP_EXPORT_LOCATION=/tmp/${BACKUP_FILENAME}
BACKUP_ENCRYPT_LOCATION=/backups/${BACKUP_FILENAME}
BACKUP_KEY_LOCATION=/tmp/backup.key
BACKUP_BUCKET_URI=s3://jm-argo-test

export_argocd () {
    echo "exporting argo-cd"
    create_backup
    encrypt_backup
    push_backup
    echo "argo-cd export complete"
}

create_backup () {
    echo "creating argo-cd backup"
    #argocd-util export > ${BACKUP_EXPORT_LOCATION}
    echo "exported: '`date`'" > ${BACKUP_EXPORT_LOCATION}
}

encrypt_backup () {
    echo "encrypting argo-cd backup"
    openssl enc -aes-256-cbc -pbkdf2 -pass file:${BACKUP_KEY_LOCATION} -in ${BACKUP_EXPORT_LOCATION} -out ${BACKUP_ENCRYPT_LOCATION}
    rm ${BACKUP_EXPORT_LOCATION}
}

push_backup () {
    echo "pushing argo-cd backup"
    aws s3 mb ${BACKUP_BUCKET_URI}
    aws s3 cp ${BACKUP_ENCRYPT_LOCATION} ${BACKUP_BUCKET_URI}/${BACKUP_FILENAME}
}

import_argocd () {
    echo "importing argo-cd"
    pull_backup
    decrypt_backup
    load_backup
    echo "argo-cd import complete"
}

pull_backup () {
    echo "pulling argo-cd backup"
    aws s3 cp ${BACKUP_BUCKET_URI}/${BACKUP_FILENAME} ${BACKUP_ENCRYPT_LOCATION}
}

decrypt_backup () {
    echo "decrypting argo-cd backup"
    openssl enc -aes-256-cbc -d -pbkdf2 -pass file:${BACKUP_KEY_LOCATION} -in ${BACKUP_ENCRYPT_LOCATION} -out ${BACKUP_EXPORT_LOCATION}
}

load_backup () {
    echo "loading argo-cd backup"
    #argocd-util import - < ${BACKUP_EXPORT_LOCATION}
    cat ${BACKUP_EXPORT_LOCATION}
}

usage () {
    echo "usage: $0 export|import"
}

case  $1 in
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
