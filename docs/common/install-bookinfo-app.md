[Return to Project Root](../README.md)

## Installing the Bookinfo Application

You can use the `bookinfo` example application to explore service mesh features. 
Using the `bookinfo` application, you can easily confirm that requests from a 
web browser pass through the mesh and reach the application.

The `bookinfo` application displays information about a book, similar to a 
single catalog entry of an online book store. The application displays a page 
that describes the book, lists book details (ISBN, number of pages, and other 
information), and book reviews.

The `bookinfo` application is exposed through the mesh, and the mesh configuration 
determines how the microservices comprising the application are used to serve 
requests. The review information comes from one of three services: `reviews-v1`, 
`reviews-v2` or `reviews-v3`. If you deploy the `bookinfo` application without 
defining the `reviews` virtual service, then the mesh uses a round-robin rule to 
route requests to a service.

By deploying the `reviews` virtual service, you can specify a different behavior. 
For example, you can specify that if a user logs into the `bookinfo` application, 
then the mesh routes requests to the `reviews-v2` service, and the application 
displays reviews with black stars. If a user does not log into the `bookinfo` 
application, then the mesh routes requests to the `reviews-v3` service, and the 
application displays reviews with red stars.

For more information, see [Bookinfo Application](https://istio.io/latest/docs/examples/bookinfo/) in the upstream Istio documentation.

After following the instructions for [Deploying the application](https://istio.io/latest/docs/examples/bookinfo/#start-the-application-services), **you 
will need to create and configure a gateway** for the `bookinfo` application to 
be accessible outside the cluster.
