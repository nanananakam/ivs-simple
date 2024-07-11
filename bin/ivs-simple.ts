#!/usr/bin/env node
import 'source-map-support/register';
import * as cdk from 'aws-cdk-lib';
import { IvsSimpleStack } from '../lib/ivs-simple-stack';

const app = new cdk.App();
new IvsSimpleStack(app, 'IvsSimpleStack', {
    env: { region: 'ap-northeast-1' },
});
