#!/usr/bin/env node
import { App } from 'aws-cdk-lib';

import { FargateServiceStack } from '../lib/FargateServiceStack';


const app = new App();
const fargateStack = new FargateServiceStack(app, 'SqsFargateServiceStack');

