// Code generated by protoc-gen-tstypes. DO NOT EDIT.

declare namespace routeguide {

    export interface Point {
        latitude?: number;
        longitude?: number;
    }

    export interface Rectangle {
        lo?: Point;
        hi?: Point;
    }

    export interface Feature {
        name?: string;
        location?: Point;
    }

    export interface RouteNote {
        location?: Point;
        message?: string;
    }

    export interface RouteSummary {
        pointCount?: number;
        featureCount?: number;
        distance?: number;
        elapsedTime?: number;
    }

    export interface RouteGuideService {
        GetFeature: (r:Point) => Feature;
        ListFeatures: (r:Rectangle) => AsyncIterator<Feature>;
        RecordRoute: (r:AsyncIterator<Point>) => RouteSummary;
        RouteChat: (r:AsyncIterator<RouteNote>) => AsyncIterator<RouteNote>;
    }
}
