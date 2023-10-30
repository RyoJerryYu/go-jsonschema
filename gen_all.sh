#!/bin/bash

# Generate all the files for the schema v2

readonly ROOT=$PWD
readonly SCHEMA_DIR=$ROOT/schema/v2

go install $ROOT/cmd/jsonschemagen

function gen_one() {
    local FILE=$1
    local DIR=$(dirname $FILE)
    local NAME=$(basename $FILE .json)
    local OUT_DIR=$ROOT/out

    if [[ $OUT_DIR != $SCHEMA_DIR ]]; then
        local RELATIVE_DIR=${DIR#$SCHEMA_DIR/}
        OUT_DIR=$OUT_DIR/$RELATIVE_DIR
    fi

    OUT_DIR=$(echo $OUT_DIR | sed 's/\$//g')

    mkdir -p $OUT_DIR

    local OUT_FILE=$OUT_DIR/$NAME.schema.go
    
    jsonschemagen -s $FILE -o $OUT_FILE
}

for FILE in $(find -L $SCHEMA_DIR -name '*.json'); do
    gen_one $FILE
done