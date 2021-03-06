// Code generated by protoc-gen-tstypes. DO NOT EDIT.

declare namespace example_with_field_options {

    export enum SearchRequest_Corpus {
        UNIVERSAL = "UNIVERSAL",
        WEB = "WEB",
        IMAGES = "IMAGES",
        LOCAL = "LOCAL",
        NEWS = "NEWS",
        PRODUCTS = "PRODUCTS",
        VIDEO = "VIDEO",
    }
    export interface SearchRequest_XyzEntry {
        key?: string;
        value?: number;
    }

    // SearchRequest is an example type representing a search query.
    export interface SearchRequest {
        query?: string;
        pageNumber?: number;
        // Number of results per page.
        resultPerPage?: number; // Should never be zero.
        corpus?: SearchRequest_Corpus;
        sentAt?: google.protobuf.Timestamp;
        xyz?: { [key: string]: number };
        zytes?: Uint8Array;
        exampleRequired: number;
    }

    export interface SearchResponse {
        results: Array<string>;
        numResults: number;
        originalRequest: SearchRequest;
        nextResultsUri?: string;
    }

}

