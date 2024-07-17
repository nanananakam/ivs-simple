import * as cdk from 'aws-cdk-lib';
import {Construct} from 'constructs';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import {FunctionUrlAuthType, HttpMethod} from 'aws-cdk-lib/aws-lambda';
import * as iam from 'aws-cdk-lib/aws-iam';
import * as path from "node:path";
import * as dynamodb from 'aws-cdk-lib/aws-dynamodb';
import {aws_ecr_assets} from "aws-cdk-lib";

export class IvsSimpleStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    const dynamoDbTable = new dynamodb.Table(this, 'IvsSimpleTable', {
      partitionKey: { name: 'arn', type: dynamodb.AttributeType.STRING },
      removalPolicy: cdk.RemovalPolicy.DESTROY,
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
    });

    // Lambda function
    const lambdaFunction = new lambda.DockerImageFunction(this, 'IvsSimpleFunction', {
      code: lambda.DockerImageCode.fromImageAsset(path.resolve(__dirname, '../app'),{
        platform: aws_ecr_assets.Platform.LINUX_ARM64,
      }),
      memorySize: 128,
      timeout: cdk.Duration.seconds(30),
      environment: {
        REGION: process.env.region || "",
        TABLE_NAME: dynamoDbTable.tableName,
      },
      architecture: lambda.Architecture.ARM_64,
    });


    // Create a policy statement for IVS access
    const policy = new iam.PolicyStatement({
      actions: ["ivs:*","cloudwatch:*","dynamodb:*","ivschat:*"],
      resources: ["*"]
    });

    lambdaFunction.addToRolePolicy(policy);

    const functionUrl = lambdaFunction.addFunctionUrl({
      authType: FunctionUrlAuthType.NONE,
      cors: {
        allowedOrigins: ['*'],
        allowedMethods: [HttpMethod.GET,HttpMethod.POST],
      },
    });

    // Output the Function URL
    new cdk.CfnOutput(this, 'FunctionUrlOutput', {
      value: functionUrl.url,
    });
  }
}
